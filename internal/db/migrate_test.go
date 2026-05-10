package db

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate_AppliesAllMigrationsExactlyOnce(t *testing.T) {
	t.Parallel()

	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, Migrate(context.Background(), d))

	// schema_migrations should have version 4 recorded
	// (0001_initial + 0002_configuration + 0003_polling_discipline + 0004_extraction).
	var version int
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 4, version)

	// subscriptions table should exist (introduced in 0001).
	_, err = d.Exec("INSERT INTO subscriptions (title, feed_url, next_poll_at, created_at) VALUES (?, ?, ?, ?)",
		"x", "https://example.com/feed", 0, 0)
	require.NoError(t, err)

	// Re-running Migrate must be a no-op.
	require.NoError(t, Migrate(context.Background(), d))
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 4, version)
}

func TestMigrate_AddsVelocityColumn(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	var name string
	var defaultVal sql.NullString
	err := d.QueryRowContext(ctx, `
		SELECT name, "dflt_value" FROM pragma_table_info('subscriptions')
		WHERE name = 'velocity_24h_x100'
	`).Scan(&name, &defaultVal)
	require.NoError(t, err, "velocity_24h_x100 column not found")
	require.Equal(t, "0", defaultVal.String, "default value")
}

func TestMigrate_AddsExtractionColumns(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	cases := []struct {
		table, column, def string
	}{
		{"subscriptions", "extract", "0"},
		{"subscriptions", "extract_selector", "''"},
		{"entries", "extract_failed", "0"},
	}
	for _, c := range cases {
		var name string
		var defaultVal sql.NullString
		err := d.QueryRowContext(ctx,
			`SELECT name, "dflt_value" FROM pragma_table_info(?) WHERE name = ?`,
			c.table, c.column).Scan(&name, &defaultVal)
		require.NoError(t, err, "%s.%s not found", c.table, c.column)
		require.Equal(t, c.def, defaultVal.String, "%s.%s default", c.table, c.column)
	}

	// schema_migrations should advance to 4.
	var version int
	require.NoError(t, d.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version))
	require.Equal(t, 4, version)
}
