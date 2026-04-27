package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Entry is the cached representation of a proxied resource.
type Entry struct {
	Body        []byte
	ContentType string
	ETag        string
}

// Cache is the filesystem-backed proxy cache. Layout:
//
//	<dir>/<2-char hex shard>/<sha256 hex of url>
//	<dir>/<2-char hex shard>/<sha256 hex of url>.meta
//
// The .meta sidecar is a tiny JSON document holding the origin
// Content-Type and ETag.
type Cache struct {
	dir string
}

// NewCache builds a Cache rooted at dir. The directory is created
// lazily on the first Put.
func NewCache(dir string) *Cache { return &Cache{dir: dir} }

type metaJSON struct {
	ContentType string `json:"content_type"`
	ETag        string `json:"etag,omitempty"`
}

// urlPath returns the body path for url. Layout:
//
//	<dir>/<2-char hex shard>/<sha256 hex of url>
func (c *Cache) urlPath(url string) string {
	sum := sha256.Sum256([]byte(url))
	hexsum := hex.EncodeToString(sum[:])
	return filepath.Join(c.dir, hexsum[:2], hexsum)
}

// Get returns the cached entry for url, ok=false if missing.
//
// A missing or unparseable .meta sidecar is treated as a cache miss
// (and the orphaned body is removed) so we never serve a response with
// an empty Content-Type. Both files are written best-effort by Put,
// without an fsync, so a crash between the two writes can leave a body
// without meta — refetching from origin is the safe recovery.
//
// Reading touches the body's mtime so age-based eviction tracks
// last-access rather than last-write.
func (c *Cache) Get(url string) (Entry, bool, error) {
	bodyPath := c.urlPath(url)
	body, err := os.ReadFile(bodyPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Entry{}, false, nil
		}
		return Entry{}, false, err
	}
	metaRaw, err := os.ReadFile(bodyPath + ".meta")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Body without meta — likely an interrupted write. Drop both
			// and force a refetch.
			_ = os.Remove(bodyPath)
			return Entry{}, false, nil
		}
		return Entry{}, false, err
	}
	var m metaJSON
	if err := json.Unmarshal(metaRaw, &m); err != nil {
		// Corrupt sidecar — same recovery as a missing one.
		_ = os.Remove(bodyPath)
		_ = os.Remove(bodyPath + ".meta")
		return Entry{}, false, nil
	}
	now := nowFn()
	_ = os.Chtimes(bodyPath, now, now)
	return Entry{Body: body, ContentType: m.ContentType, ETag: m.ETag}, true, nil
}

// Put writes (overwrites) the cached body and .meta. Best-effort
// fsync is intentionally skipped — the cache is recoverable from the
// origin.
func (c *Cache) Put(url string, body []byte, contentType, etag string) error {
	bodyPath := c.urlPath(url)
	if err := os.MkdirAll(filepath.Dir(bodyPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(bodyPath, body, 0o644); err != nil {
		return err
	}
	meta, err := json.Marshal(metaJSON{ContentType: contentType, ETag: etag})
	if err != nil {
		return err
	}
	return os.WriteFile(bodyPath+".meta", meta, 0o644)
}

// Evict deletes oldest-mtime entries until total size <= maxBytes.
// O(N log N) per call — fine for inline use after a Put or for a
// daily sweep. Walks only body files (.meta sidecars are removed
// alongside their bodies).
func (c *Cache) Evict(maxBytes int64) error {
	type fileInfo struct {
		path string
		size int64
		mod  int64
	}
	var files []fileInfo
	var total int64

	err := filepath.WalkDir(c.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		// Don't count .meta files separately — they're tiny and they
		// get cleaned up alongside their body.
		if filepath.Ext(p) == ".meta" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		files = append(files, fileInfo{path: p, size: info.Size(), mod: info.ModTime().UnixNano()})
		total += info.Size()
		return nil
	})
	if err != nil {
		// Missing cache dir is not an error — nothing to evict.
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if total <= maxBytes {
		return nil
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod < files[j].mod })
	for _, f := range files {
		if total <= maxBytes {
			break
		}
		// Decrement only on successful body removal so a permission /
		// race / EROFS error doesn't trick us into "stopping early"
		// when the cache is still over the cap.
		if err := os.Remove(f.path); err != nil {
			continue
		}
		_ = os.Remove(f.path + ".meta")
		total -= f.size
	}
	return nil
}

// nowFn is overridable for testing.
var nowFn = osNow
