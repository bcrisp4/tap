package proxy_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/stretchr/testify/require"
)

func newCache(t *testing.T, capBytes int64) (*proxy.Cache, string) {
	t.Helper()
	dir := t.TempDir()
	return proxy.NewCache(dir, capBytes), dir
}

func TestCache_MissThenHit(t *testing.T) {
	t.Parallel()
	c, dir := newCache(t, 1<<20)

	var calls int32
	fetch := func(ctx context.Context) (proxy.FetchedResource, error) {
		atomic.AddInt32(&calls, 1)
		return proxy.FetchedResource{
			Bytes:       []byte("hello"),
			ContentType: "image/png",
			ETag:        `"abc"`,
		}, nil
	}

	got, err := c.Get(context.Background(), "abcdef", fetch)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), got.Bytes)
	require.Equal(t, "image/png", got.ContentType)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))

	// Files should exist on disk in the sharded layout.
	binPath := filepath.Join(dir, "ab", "abcdef.bin")
	metaPath := filepath.Join(dir, "ab", "abcdef.meta")
	require.FileExists(t, binPath)
	require.FileExists(t, metaPath)

	// Second call: hit, no fetcher invocation.
	got2, err := c.Get(context.Background(), "abcdef", fetch)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), got2.Bytes)
	require.Equal(t, "image/png", got2.ContentType)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls), "fetcher must not run on hit")
}

func TestCache_FetchErrorPropagates(t *testing.T) {
	t.Parallel()
	c, dir := newCache(t, 1<<20)

	wantErr := errors.New("origin down")
	_, err := c.Get(context.Background(), "deadbeef", func(ctx context.Context) (proxy.FetchedResource, error) {
		return proxy.FetchedResource{}, wantErr
	})
	require.ErrorIs(t, err, wantErr)

	// No files should be written on fetch failure.
	require.NoFileExists(t, filepath.Join(dir, "de", "deadbeef.bin"))
	require.NoFileExists(t, filepath.Join(dir, "de", "deadbeef.meta"))
}

func TestCache_OrphanBinTreatsAsMiss(t *testing.T) {
	t.Parallel()
	c, dir := newCache(t, 1<<20)

	// Plant a .bin without a .meta — simulates crash between renames.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "ab"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ab", "abcdef.bin"), []byte("stale"), 0o644))

	var calls int32
	got, err := c.Get(context.Background(), "abcdef", func(ctx context.Context) (proxy.FetchedResource, error) {
		atomic.AddInt32(&calls, 1)
		return proxy.FetchedResource{Bytes: []byte("fresh"), ContentType: "image/png"}, nil
	})
	require.NoError(t, err)
	require.Equal(t, []byte("fresh"), got.Bytes)
	require.Equal(t, int32(1), atomic.LoadInt32(&calls))
}

func TestCache_RejectsInvalidHash(t *testing.T) {
	t.Parallel()
	c, dir := newCache(t, 1<<20)

	cases := []string{
		"../evil",  // path traversal
		"AB",       // upper-case hex (we accept lower only)
		"g",        // too short AND non-hex
		"ab/cd",    // slash
		"ab.cd",    // dot
	}
	var calls int32
	fetch := func(ctx context.Context) (proxy.FetchedResource, error) {
		atomic.AddInt32(&calls, 1)
		return proxy.FetchedResource{Bytes: []byte("x"), ContentType: "image/png"}, nil
	}
	for _, hash := range cases {
		_, err := c.Get(context.Background(), hash, fetch)
		require.Error(t, err, "hash %q must be rejected", hash)
	}
	require.Equal(t, int32(0), atomic.LoadInt32(&calls), "fetcher must not run for invalid hash")
	// No files should have been written anywhere under dir.
	entries, _ := os.ReadDir(dir)
	require.Empty(t, entries, "no files should have been written")
}

var _ = sync.Mutex{} // keeps the import even when later tests are added
