package db_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
)

func TestOpen_FileDB_AppliesPragmas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	defer d.Close()

	var fk int
	require.NoError(t, d.QueryRow("PRAGMA foreign_keys").Scan(&fk))
	require.Equal(t, 1, fk, "foreign_keys must be ON")

	var jm string
	require.NoError(t, d.QueryRow("PRAGMA journal_mode").Scan(&jm))
	require.Equal(t, "wal", jm, "journal_mode must be WAL")
}

func TestOpen_BadPathErrors(t *testing.T) {
	_, err := db.Open("/no/such/dir/tap.db")
	require.Error(t, err)
}
