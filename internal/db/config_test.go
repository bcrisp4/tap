package db_test

import (
	"context"
	"testing"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func TestMigrate_AddsConfigurationTable(t *testing.T) {
	t.Parallel()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))

	var name string
	err = d.QueryRowContext(context.Background(),
		`SELECT name FROM sqlite_master WHERE type='table' AND name='configuration'`).Scan(&name)
	require.NoError(t, err)
	require.Equal(t, "configuration", name)
}

func TestSetConfigIfAbsent_InsertsAndIsIdempotent(t *testing.T) {
	t.Parallel()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))

	got, err := db.SetConfigIfAbsent(context.Background(), d, "k", []byte("v1"))
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), got)

	// Second call with a different value must not overwrite.
	got2, err := db.SetConfigIfAbsent(context.Background(), d, "k", []byte("v2"))
	require.NoError(t, err)
	require.Equal(t, []byte("v1"), got2)
}

func TestGetConfig_HitAndMiss(t *testing.T) {
	t.Parallel()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))

	val, ok, err := db.GetConfig(context.Background(), d, "missing")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, val)

	_, err = db.SetConfigIfAbsent(context.Background(), d, "found", []byte("x"))
	require.NoError(t, err)

	val, ok, err = db.GetConfig(context.Background(), d, "found")
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []byte("x"), val)
}
