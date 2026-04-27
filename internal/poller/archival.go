package poller

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/bcrisp4/tap/internal/storage"
)

// ProxyCacheSweep parameterises the proxy-cache age sweep that runs
// alongside entry archival. A nil *ProxyCacheSweep skips the sweep.
type ProxyCacheSweep struct {
	Dir    string
	MaxAge time.Duration
}

// ArchiveOnce runs one archival sweep:
//  1. Tombstone + delete read+unsaved entries older than archiveAge.
//  2. (Optional) Delete proxy cache files older than ProxyCacheSweep.MaxAge.
//
// Step 1 is one IMMEDIATE transaction so a tombstone is never written
// without its DELETE landing too. Step 2 is best-effort.
func ArchiveOnce(ctx context.Context, s *storage.Store, archiveAge time.Duration, proxyCache *ProxyCacheSweep) error {
	if err := archiveEntries(ctx, s, archiveAge); err != nil {
		return err
	}
	if proxyCache != nil {
		_ = sweepProxyCache(proxyCache.Dir, proxyCache.MaxAge)
	}
	return nil
}

func archiveEntries(ctx context.Context, s *storage.Store, age time.Duration) error {
	threshold := time.Now().Add(-age).Unix()

	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	// Tombstone first (subquery snapshot), then delete. INSERT OR
	// IGNORE so a re-run on already-tombstoned rows is a no-op.
	if _, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO entry_tombstones(feed_id, hash, created_at)
		SELECT feed_id, hash, unixepoch()
		FROM entries
		WHERE read = 1 AND saved = 0 AND created_at < ?`, threshold); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM entries
		WHERE read = 1 AND saved = 0 AND created_at < ?`, threshold); err != nil {
		return err
	}
	return tx.Commit()
}

func sweepProxyCache(dir string, maxAge time.Duration) error {
	if dir == "" || maxAge <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-maxAge)
	return filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			_ = os.Remove(p)
		}
		return nil
	})
}
