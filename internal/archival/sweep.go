package archival

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/bcrisp4/tap/internal/db"
)

const chunkSize = 1000

// dbPass is the production entry point. For tests that need to count chunk
// transactions, use dbPassWithHook.
func dbPass(ctx context.Context, d *sql.DB, horizonUnix, nowUnix int64) (deleted, tombstoned int, err error) {
	return dbPassWithHook(ctx, d, horizonUnix, nowUnix, nil)
}

// dbPassWithHook runs the DB archival pass, calling onChunk (if non-nil) after
// each chunk transaction commits. Used by TestDBPass_Chunking to verify
// transaction count without exposing a test seam on the production path.
func dbPassWithHook(ctx context.Context, d *sql.DB, horizonUnix, nowUnix int64, onChunk func()) (deleted, tombstoned int, err error) {
	for {
		rows, err := db.ListArchivable(ctx, d, horizonUnix, chunkSize)
		if err != nil {
			return deleted, tombstoned, fmt.Errorf("list archivable: %w", err)
		}
		if len(rows) == 0 {
			return deleted, tombstoned, nil
		}

		ids := make([]int64, len(rows))
		tbs := make([]db.NewTombstone, len(rows))
		for i, r := range rows {
			ids[i] = r.ID
			tbs[i] = db.NewTombstone{SubscriptionID: r.SubscriptionID, EntryHash: r.Hash, DeletedAt: nowUnix}
		}

		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return deleted, tombstoned, fmt.Errorf("begin tx: %w", err)
		}

		if err := db.InsertTombstones(ctx, tx, tbs); err != nil {
			_ = tx.Rollback()
			return deleted, tombstoned, err
		}

		res, err := tx.ExecContext(ctx, buildDeleteByIDs(ids))
		if err != nil {
			_ = tx.Rollback()
			return deleted, tombstoned, fmt.Errorf("delete entries chunk: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return deleted, tombstoned, fmt.Errorf("commit chunk: %w", err)
		}

		n, _ := res.RowsAffected()
		deleted += int(n)
		tombstoned += len(tbs)

		if onChunk != nil {
			onChunk()
		}
	}
}

// buildDeleteByIDs constructs a DELETE for a slice of internal int64 IDs.
// IDs come from our own DB rows, not user input, so direct interpolation is safe.
func buildDeleteByIDs(ids []int64) string {
	if len(ids) == 0 {
		return "DELETE FROM entries WHERE 1=0"
	}
	q := "DELETE FROM entries WHERE id IN ("
	for i, id := range ids {
		if i > 0 {
			q += ","
		}
		q += fmt.Sprintf("%d", id)
	}
	return q + ")"
}

// cacheFileMeta mirrors the JSON sidecar written by internal/proxy.Cache.
// Must match M3's sidecar shape exactly.
type cacheFileMeta struct {
	ContentType string `json:"content_type"`
	ETag        string `json:"etag,omitempty"`
	ByteCount   int64  `json:"byte_count"`
	FetchedAt   int64  `json:"fetched_at"`
}

// fsPass walks the cache directory and evicts:
//  1. .meta/.bin pairs whose fetched_at is older than ageCapUnix
//  2. orphan .bin files (no corresponding .meta sidecar)
//
// ENOENT on either file during deletion is not an error (may have been
// concurrently evicted by M3's inline LRU). FS errors are logged at WARN
// but do not abort the pass.
func fsPass(cacheDir string, ageCapUnix int64, onEvict func(int)) (evicted int, err error) {
	type candidate struct{ binPath, metaPath string }
	var candidates []candidate
	var orphanBins []string
	metaBases := map[string]bool{}

	walkErr := filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil {
			if errors.Is(werr, os.ErrNotExist) {
				return nil
			}
			return werr
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		base := path[:len(path)-len(ext)]
		if ext != ".meta" {
			return nil
		}
		metaBases[base] = true
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			if !errors.Is(rerr, os.ErrNotExist) {
				slog.Warn("archival: read meta sidecar", "path", path, "err", rerr)
			}
			return nil
		}
		var meta cacheFileMeta
		if jerr := json.Unmarshal(data, &meta); jerr != nil {
			slog.Warn("archival: decode meta sidecar", "path", path, "err", jerr)
			return nil
		}
		if meta.FetchedAt < ageCapUnix {
			candidates = append(candidates, candidate{binPath: base + ".bin", metaPath: path})
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, os.ErrNotExist) {
		return 0, fmt.Errorf("walk cache dir: %w", walkErr)
	}

	_ = filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, werr error) error {
		if werr != nil || d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".bin" {
			return nil
		}
		base := path[:len(path)-len(".bin")]
		if !metaBases[base] {
			orphanBins = append(orphanBins, path)
		}
		return nil
	})

	for _, c := range candidates {
		if rerr := os.Remove(c.binPath); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			slog.Warn("archival: remove bin", "path", c.binPath, "err", rerr)
		}
		if rerr := os.Remove(c.metaPath); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			slog.Warn("archival: remove meta", "path", c.metaPath, "err", rerr)
		}
		evicted++
	}
	for _, path := range orphanBins {
		if rerr := os.Remove(path); rerr != nil && !errors.Is(rerr, os.ErrNotExist) {
			slog.Warn("archival: remove orphan bin", "path", path, "err", rerr)
		}
		evicted++
	}

	if onEvict != nil && evicted > 0 {
		onEvict(evicted)
	}
	return evicted, nil
}

// ExportedDBPass is a test-only shim for cmd/tap/main_test.go.
func ExportedDBPass(ctx context.Context, d *sql.DB, horizonUnix, nowUnix int64) (int, int, error) {
	return dbPass(ctx, d, horizonUnix, nowUnix)
}

// ExportedFSPass is a test-only shim for cmd/tap/main_test.go.
func ExportedFSPass(cacheDir string, ageCapUnix int64, onEvict func(int)) (int, error) {
	return fsPass(cacheDir, ageCapUnix, onEvict)
}
