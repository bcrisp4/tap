package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate_AppliesAllMigrationsExactlyOnce(t *testing.T) {
	t.Parallel()

	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, Migrate(context.Background(), d))

	// schema_migrations should have version 1 recorded.
	var version int
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 1, version)

	// subscriptions table should exist (introduced in 0001).
	_, err = d.Exec("INSERT INTO subscriptions (title, feed_url, next_poll_at, created_at) VALUES (?, ?, ?, ?)",
		"x", "https://example.com/feed", 0, 0)
	require.NoError(t, err)

	// Re-running Migrate must be a no-op.
	require.NoError(t, Migrate(context.Background(), d))
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 1, version)
}
