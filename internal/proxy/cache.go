package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// FetchedResource is what a fetcher closure returns. Bytes is the full body
// (caller is responsible for body cap); ContentType is the canonical sniffed
// type from validateImage; ETag is the origin's value, stored for forward
// compatibility (M3 doesn't act on it).
type FetchedResource struct {
	Bytes       []byte
	ContentType string
	ETag        string
}

// Cache is an FS-backed LRU cache for proxy responses. Bytes live in
// <dir>/<aa>/<hash>.bin; metadata in a sibling .meta JSON file.
// Eviction is mtime-based and fires inline when adding a new entry would
// exceed capBytes. Concurrent misses for the same hash collapse via singleflight.
type Cache struct {
	dir      string
	capBytes int64
	sf       singleflight.Group
	evictMu  sync.Mutex
}

// NewCache returns a Cache rooted at dir with a storage cap of capBytes.
// The dir need not exist; it is created on first write.
func NewCache(dir string, capBytes int64) *Cache {
	return &Cache{dir: dir, capBytes: capBytes}
}

type sidecar struct {
	ContentType string `json:"content_type"`
	ETag        string `json:"etag,omitempty"`
	ByteCount   int64  `json:"byte_count"`
	FetchedAt   int64  `json:"fetched_at"`
}

// Get returns cached bytes (hit) or invokes fetch (miss), caches the result,
// and returns the bytes. Concurrent misses for the same hash run fetch once.
// Get returns an error immediately if hash fails the lowercase-hex validation
// — the fetcher is never invoked for invalid hashes.
func (c *Cache) Get(ctx context.Context, hash string, fetch func(ctx context.Context) (FetchedResource, error)) (FetchedResource, error) {
	if _, _, dir := c.paths(hash); dir == "" {
		return FetchedResource{}, fmt.Errorf("proxy cache: invalid hash %q", hash)
	}
	if got, ok := c.tryHit(hash); ok {
		return got, nil
	}

	v, err, _ := c.sf.Do(hash, func() (any, error) {
		// Recheck under singleflight in case another goroutine just filled the cache.
		if got, ok := c.tryHit(hash); ok {
			return got, nil
		}
		got, err := fetch(ctx)
		if err != nil {
			return FetchedResource{}, err
		}
		if err := c.write(hash, got); err != nil {
			return FetchedResource{}, fmt.Errorf("cache write: %w", err)
		}
		return got, nil
	})
	if err != nil {
		return FetchedResource{}, err
	}
	return v.(FetchedResource), nil
}

func (c *Cache) paths(hash string) (binPath, metaPath, dirPath string) {
	if len(hash) < 2 {
		return "", "", ""
	}
	for _, r := range hash {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return "", "", ""
		}
	}
	dirPath = filepath.Join(c.dir, hash[:2])
	binPath = filepath.Join(dirPath, hash+".bin")
	metaPath = filepath.Join(dirPath, hash+".meta")
	return
}

func (c *Cache) tryHit(hash string) (FetchedResource, bool) {
	binPath, metaPath, _ := c.paths(hash)
	if binPath == "" {
		return FetchedResource{}, false
	}
	bin, err := os.ReadFile(binPath)
	if err != nil {
		return FetchedResource{}, false
	}
	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return FetchedResource{}, false
	}
	var sc sidecar
	if err := json.Unmarshal(metaBytes, &sc); err != nil {
		return FetchedResource{}, false
	}
	return FetchedResource{
		Bytes:       bin,
		ContentType: sc.ContentType,
		ETag:        sc.ETag,
	}, true
}

func (c *Cache) write(hash string, res FetchedResource) error {
	binPath, metaPath, dirPath := c.paths(hash)
	if dirPath == "" {
		return fmt.Errorf("proxy cache: invalid hash %q", hash)
	}
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return err
	}

	// binTmp and metaTmp may survive a crash before the rename below. The
	// next Get treats the absent .bin / .meta as a miss and overwrites them.
	// Startup cleanup of *.tmp files is a future operator convenience.
	binTmp := binPath + ".tmp"
	metaTmp := metaPath + ".tmp"
	if err := os.WriteFile(binTmp, res.Bytes, 0o644); err != nil {
		return err
	}

	sc := sidecar{
		ContentType: res.ContentType,
		ETag:        res.ETag,
		ByteCount:   int64(len(res.Bytes)),
		FetchedAt:   time.Now().Unix(),
	}
	scBytes, err := json.Marshal(sc)
	if err != nil {
		_ = os.Remove(binTmp)
		return err
	}
	if err := os.WriteFile(metaTmp, scBytes, 0o644); err != nil {
		_ = os.Remove(binTmp)
		return err
	}

	if err := os.Rename(binTmp, binPath); err != nil {
		_ = os.Remove(binTmp)
		_ = os.Remove(metaTmp)
		return err
	}
	if err := os.Rename(metaTmp, metaPath); err != nil {
		_ = os.Remove(metaTmp)
		_ = os.Remove(binPath)
		return err
	}
	return nil
}
