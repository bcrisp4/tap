package tracing_test

import (
	"context"
	"testing"

	"github.com/bcrisp4/tap/internal/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_NoEndpoint_NoopProvider(t *testing.T) {
	err := tracing.Init(tracing.Opts{})
	require.NoError(t, err)
	defer tracing.Shutdown(context.Background())
}

func TestInit_Shutdown_Clean(t *testing.T) {
	_ = tracing.Init(tracing.Opts{})
	err := tracing.Shutdown(context.Background())
	assert.NoError(t, err)
}
