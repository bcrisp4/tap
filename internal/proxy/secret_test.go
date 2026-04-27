package proxy_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/bcrisp4/tap/internal/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return storage.New(d)
}

func TestEnsureSecret_GeneratesAndPersists(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	secret1, err := proxy.EnsureSecret(ctx, s)
	require.NoError(t, err)
	require.Len(t, secret1, 64, "32 bytes hex-encoded = 64 chars")

	// Idempotent: second call returns the same value.
	secret2, err := proxy.EnsureSecret(ctx, s)
	require.NoError(t, err)
	require.Equal(t, secret1, secret2)
}

// TestEnsureSecret_ConcurrentCallersAgree confirms that even if every
// goroutine takes the "ErrNotFound -> generate -> insert" path, all
// callers return the same persisted secret. INSERT OR IGNORE ensures
// the first writer wins; the post-insert re-read reconciles the rest.
func TestEnsureSecret_ConcurrentCallersAgree(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	const n = 16
	results := make([]string, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			got, err := proxy.EnsureSecret(ctx, s)
			require.NoError(t, err)
			results[i] = got
		}(i)
	}
	wg.Wait()

	for i := 1; i < n; i++ {
		require.Equal(t, results[0], results[i], "all callers must converge on one secret")
	}
	require.Len(t, results[0], 64)
}
