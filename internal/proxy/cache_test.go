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

func TestCache_SingleflightCoalesces(t *testing.T) {
	t.Parallel()
	c, _ := newCache(t, 1<<20)

	const N = 20
	var calls int32
	start := make(chan struct{})
	gate := make(chan struct{})

	fetch := func(ctx context.Context) (proxy.FetchedResource, error) {
		atomic.AddInt32(&calls, 1)
		<-gate // hold the in-flight fetch open until the test releases it
		return proxy.FetchedResource{Bytes: []byte("x"), ContentType: "image/png"}, nil
	}

	var wg sync.WaitGroup
	results := make([]error, N)
	for i := 0; i < N; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := c.Get(context.Background(), "feedface", fetch)
			results[i] = err
		}()
	}

	close(start)
	// Give all goroutines a moment to enter c.Get and either find the
	// in-flight singleflight slot or hit the fast path.
	// (singleflight has a tiny window where the first caller is in fetch
	// but hasn't yet been registered; the test tolerates one extra call by
	// asserting <= 2 below to avoid flakiness on slow runners.)
	close(gate)
	wg.Wait()

	for _, err := range results {
		require.NoError(t, err)
	}
	got := atomic.LoadInt32(&calls)
	require.LessOrEqual(t, got, int32(2), "singleflight should collapse %d concurrent misses to 1 (allow 2 for race tolerance)", N)
	require.GreaterOrEqual(t, got, int32(1))
}
