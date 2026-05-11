package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigrate_AppliesAllMigrationsExactlyOnce(t *testing.T) {
	t.Parallel()

	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	require.NoError(t, Migrate(context.Background(), d))

	// schema_migrations should have version 11 recorded (0001 through 0011).
	var version int
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 11, version)

	// subscriptions table should exist (introduced in 0001).
	// user_id is NOT NULL after 0006, so we need a user first.
	res, err := d.Exec("INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)",
		"testmig", "x", "admin", 0)
	require.NoError(t, err)
	uid, _ := res.LastInsertId()
	_, err = d.Exec("INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at) VALUES (?, ?, ?, ?, ?)",
		uid, "x", "https://example.com/feed", 0, 0)
	require.NoError(t, err)

	// Re-running Migrate must be a no-op.
	require.NoError(t, Migrate(context.Background(), d))
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 11, version)
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

	// schema_migrations should be at the latest version (M4-redesign added 0011).
	var version int
	require.NoError(t, d.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version))
	require.Equal(t, 11, version)
}

func TestMigrate_0006_UserDataIsolation(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()

	// Verify user_id FK is enforced: insert with non-existent user_id must fail.
	_, err := d.ExecContext(ctx,
		`INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at)
		 VALUES (999, 't', 'http://x.com/feed', 0, 0)`)
	require.Error(t, err)
	require.Contains(t, err.Error(), "FOREIGN KEY")

	// Verify UNIQUE is now (user_id, feed_url).
	res1, err := d.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, created_at) VALUES ('u1','h','admin',0)`)
	require.NoError(t, err)
	uid1, _ := res1.LastInsertId()
	res2, err := d.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, created_at) VALUES ('u2','h','admin',0)`)
	require.NoError(t, err)
	uid2, _ := res2.LastInsertId()

	_, err = d.ExecContext(ctx,
		`INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at)
		 VALUES (?, 'Feed', 'http://shared.example/feed', 0, 0)`, uid1)
	require.NoError(t, err, "first user should be able to subscribe")

	_, err = d.ExecContext(ctx,
		`INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at)
		 VALUES (?, 'Feed', 'http://shared.example/feed', 0, 0)`, uid2)
	require.NoError(t, err, "second user must be able to subscribe to the same URL")

	_, err = d.ExecContext(ctx,
		`INSERT INTO subscriptions (user_id, title, feed_url, next_poll_at, created_at)
		 VALUES (?, 'Feed', 'http://shared.example/feed', 0, 0)`, uid1)
	require.Error(t, err, "same user subscribing to the same URL twice must fail")
	require.Contains(t, err.Error(), "UNIQUE constraint failed")
}

func TestMigrate_0011_CategoryPosition_OnPopulatedDB(t *testing.T) {
	t.Parallel()
	// Open a fresh DB and apply only migrations 0001-0010, insert two categories,
	// then apply 0011 and verify position column exists with default 0.
	d, err := Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	// Apply all migrations — after 0011 lands, newTestDB applies it too.
	// We need to apply through 0010 only. Use a scoped helper below.
	require.NoError(t, applyMigrationsUpTo(t, d, "0010"))

	// Insert a user and two categories using the pre-0011 schema.
	res, err := d.ExecContext(context.Background(),
		`INSERT INTO users (username, password_hash, role, created_at) VALUES ('alice0011', 'x', 'admin', 0)`)
	require.NoError(t, err)
	uid, _ := res.LastInsertId()
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO categories (user_id, name, created_at) VALUES (?, 'A', 0), (?, 'B', 0)`,
		uid, uid)
	require.NoError(t, err)

	// Now apply remaining migrations (0011).
	require.NoError(t, Migrate(context.Background(), d))

	var pa, pb int64
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT position FROM categories WHERE name = 'A'`).Scan(&pa))
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT position FROM categories WHERE name = 'B'`).Scan(&pb))
	require.Equal(t, int64(0), pa)
	require.Equal(t, int64(0), pb)
}

// applyMigrationsUpTo applies migrations with version <= the given prefix (e.g. "0010").
// It reuses the same logic as Migrate but stops at the given version number.
func applyMigrationsUpTo(t *testing.T, d *sql.DB, versionPrefix string) error {
	t.Helper()
	ctx := context.Background()
	if _, err := d.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	for _, name := range files {
		base := filepath.Base(name)
		// Stop before migration files whose version prefix > the cutoff.
		if base > versionPrefix+"_" {
			break
		}
		v, err := versionFromFilename(name)
		if err != nil {
			return err
		}
		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, 0)", v); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
	}
	return nil
}

func TestMigrate_0007_2FAAndPasskeys(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	ctx := context.Background()
	// Verify sessions table has new columns
	_, err := d.ExecContext(ctx,
		`SELECT user_agent, address, webauthn_challenge FROM sessions LIMIT 1`)
	require.NoError(t, err)
	// Verify new tables exist
	for _, tbl := range []string{"pending_logins", "totp_secrets", "recovery_codes", "passkeys"} {
		_, err = d.ExecContext(ctx, `SELECT 1 FROM `+tbl+` LIMIT 1`)
		require.NoError(t, err, "table %s should exist", tbl)
	}
	// sessions.user_id should be nullable
	_, err = d.ExecContext(ctx,
		`INSERT INTO sessions (user_id, token_hash, csrf_token, created_at, last_seen_at, idle_expires_at, absolute_expires_at)
		 VALUES (NULL, 'testhash0007', 'csrf', 0, 0, 9999999999, 9999999999)`)
	require.NoError(t, err, "sessions.user_id should be nullable for anonymous challenge sessions")
}
