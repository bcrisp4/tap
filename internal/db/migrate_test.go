package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
)

func TestMigrate_AppliesBaseline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	defer d.Close()

	require.NoError(t, db.Migrate(context.Background(), d))

	// schema_version must exist and contain the baseline version.
	var n int
	require.NoError(t, d.QueryRow("SELECT count(*) FROM schema_version WHERE version = '0001_baseline'").Scan(&n))
	require.Equal(t, 1, n)
}

func TestMigrate_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	defer d.Close()

	require.NoError(t, db.Migrate(context.Background(), d))
	require.NoError(t, db.Migrate(context.Background(), d), "second call must not error")

	var n int
	require.NoError(t, d.QueryRow("SELECT count(*) FROM schema_version").Scan(&n))
	require.Equal(t, 2, n, "every migration recorded exactly once")
}

func TestMigrate_InitialSchemaTablesExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(context.Background(), d))

	want := []string{
		"users", "categories", "icons", "feeds", "entries",
		"entry_tombstones", "enclosures", "entries_fts", "config",
	}
	for _, name := range want {
		var n int
		require.NoError(t,
			d.QueryRow("SELECT count(*) FROM sqlite_master WHERE name = ?", name).Scan(&n),
			"missing table %q", name)
		require.Equal(t, 1, n, "table %q must exist", name)
	}

	// Default user seeded.
	var username string
	require.NoError(t, d.QueryRow("SELECT username FROM users WHERE id = 1").Scan(&username))
	require.Equal(t, "default", username)
}
