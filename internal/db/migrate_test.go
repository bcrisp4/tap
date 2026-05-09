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

	// schema_migrations should have version 3 recorded
	// (0001_initial + 0002_configuration + 0003_polling_discipline).
	var version int
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 3, version)

	// subscriptions table should exist (introduced in 0001).
	_, err = d.Exec("INSERT INTO subscriptions (title, feed_url, next_poll_at, created_at) VALUES (?, ?, ?, ?)",
		"x", "https://example.com/feed", 0, 0)
	require.NoError(t, err)

	// Re-running Migrate must be a no-op.
	require.NoError(t, Migrate(context.Background(), d))
	require.NoError(t, d.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version))
	require.Equal(t, 3, version)
}

func TestMigrate_AddsVelocityColumn(t *testing.T) {
	ctx := context.Background()
	d, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := Migrate(ctx, d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	var name string
	var defaultVal sql.NullString
	err = d.QueryRowContext(ctx, `
        SELECT name, "dflt_value" FROM pragma_table_info('subscriptions')
        WHERE name = 'velocity_24h_x100'
    `).Scan(&name, &defaultVal)
	if err != nil {
		t.Fatalf("velocity_24h_x100 column not found: %v", err)
	}
	if defaultVal.String != "0" {
		t.Errorf("default = %q; want 0", defaultVal.String)
	}
}
