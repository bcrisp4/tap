package storage_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/storage"
)

// newTestStore opens a fresh on-disk SQLite database in t.TempDir,
// applies migrations, and returns a *Store. Closes the DB at test
// teardown. Used by every test file in this package.
func newTestStore(t *testing.T) *storage.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return storage.New(d)
}

func TestNew_HasUnderlyingDB(t *testing.T) {
	s := newTestStore(t)
	require.NotNil(t, s.DB())
	require.NoError(t, s.DB().Ping())
}
