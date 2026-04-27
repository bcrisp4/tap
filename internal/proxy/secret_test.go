package proxy_test

import (
	"context"
	"path/filepath"
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
