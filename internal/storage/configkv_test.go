package storage_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func TestConfigKV_GetMissing(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetConfig(context.Background(), "nope")
	require.True(t, errors.Is(err, storage.ErrNotFound))
}

func TestConfigKV_SetThenGet(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	require.NoError(t, s.SetConfig(ctx, "proxy_hmac_secret", "deadbeef"))
	v, err := s.GetConfig(ctx, "proxy_hmac_secret")
	require.NoError(t, err)
	require.Equal(t, "deadbeef", v)
}

func TestConfigKV_SetIsUpsert(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)
	require.NoError(t, s.SetConfig(ctx, "k", "v1"))
	require.NoError(t, s.SetConfig(ctx, "k", "v2"))
	v, err := s.GetConfig(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, "v2", v)
}
