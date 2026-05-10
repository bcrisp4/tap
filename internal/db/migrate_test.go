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

	// schema_migrations should have version 5 recorded
	// (0001_initial + 0002_configuration + 0003_polling_discipline +
	// 0004_extraction + 0005_auth_and_credentials).
	var version int
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 5, version)

	// subscriptions table should exist (introduced in 0001).
	_, err = d.Exec("INSERT INTO subscriptions (title, feed_url, next_poll_at, created_at) VALUES (?, ?, ?, ?)",
		"x", "https://example.com/feed", 0, 0)
	require.NoError(t, err)

	// Re-running Migrate must be a no-op.
	require.NoError(t, Migrate(context.Background(), d))
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 5, version)
}

func TestMigrate_AddsAuthTables(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	// users table presence + key columns
	for _, col := range []string{"id", "username", "password_hash", "role", "created_at", "disabled_at"} {
		var name string
		err := d.QueryRowContext(ctx,
			`SELECT name FROM pragma_table_info('users') WHERE name = ?`, col).Scan(&name)
		require.NoError(t, err, "users.%s missing", col)
	}

	// users.username UNIQUE — SQLite creates an autoindex for inline UNIQUE
	// constraints (sqlite_autoindex_users_<n>) with empty .sql, so check the
	// table CREATE statement and the index list together.
	var tableSQL string
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'users'`,
	).Scan(&tableSQL))
	require.Contains(t, tableSQL, "username", "users.username column missing in CREATE TABLE")
	require.Contains(t, tableSQL, "UNIQUE", "users table CREATE missing UNIQUE constraint")

	var idx int
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = 'users'`,
	).Scan(&idx))
	require.GreaterOrEqual(t, idx, 1, "users.username should have a UNIQUE index (autoindex)")

	// sessions table presence + key columns
	for _, col := range []string{
		"id", "user_id", "token_hash", "csrf_token",
		"created_at", "last_seen_at", "idle_expires_at", "absolute_expires_at",
	} {
		var name string
		err := d.QueryRowContext(ctx,
			`SELECT name FROM pragma_table_info('sessions') WHERE name = ?`, col).Scan(&name)
		require.NoError(t, err, "sessions.%s missing", col)
	}
}

func TestMigrate_AddsSubscriptionCredentialColumns(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	for _, c := range []struct{ col, def string }{
		{"cookie", "''"},
		{"basic_auth_user", "''"},
		{"basic_auth_pass", "''"},
	} {
		var name string
		var defaultVal sql.NullString
		err := d.QueryRowContext(ctx,
			`SELECT name, "dflt_value" FROM pragma_table_info('subscriptions') WHERE name = ?`,
			c.col).Scan(&name, &defaultVal)
		require.NoError(t, err, "subscriptions.%s not found", c.col)
		require.Equal(t, c.def, defaultVal.String, "subscriptions.%s default", c.col)
	}
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

	// schema_migrations should be at the latest version (M6 added 0005).
	var version int
	require.NoError(t, d.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version))
	require.Equal(t, 5, version)
}
