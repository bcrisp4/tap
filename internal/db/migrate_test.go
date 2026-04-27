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
	require.Equal(t, 1, n, "baseline must not be recorded twice")
}
