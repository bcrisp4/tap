package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpen_InMemory_AppliesPragmas(t *testing.T) {
	t.Parallel()

	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, d.PingContext(context.Background()))

	var fk int
	require.NoError(t, d.QueryRow("PRAGMA foreign_keys").Scan(&fk))
	require.Equal(t, 1, fk, "foreign_keys must be ON")

	var jm string
	require.NoError(t, d.QueryRow("PRAGMA journal_mode").Scan(&jm))
	// in-memory always reports "memory"; for a real file it would be "wal".
	require.Contains(t, []string{"wal", "memory"}, jm)
}
