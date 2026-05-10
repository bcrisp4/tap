package metrics_test

import (
	"context"
	"testing"

	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit_NoopBeforeInit(t *testing.T) {
	// All instrument calls before Init must not panic.
	assert.NotPanics(t, func() {
		metrics.PollsTotal.Add(context.Background(), 1)
	})
}

func TestInit_Shutdown(t *testing.T) {
	reg := prometheus.NewRegistry()
	err := metrics.InitWithRegistry(reg, metrics.Opts{})
	require.NoError(t, err)
	err = metrics.Shutdown(context.Background())
	assert.NoError(t, err)
}

func TestInit_DoubleInit(t *testing.T) {
	reg := prometheus.NewRegistry()
	err := metrics.InitWithRegistry(reg, metrics.Opts{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = metrics.Shutdown(context.Background()) })

	err = metrics.InitWithRegistry(prometheus.NewRegistry(), metrics.Opts{})
	assert.Error(t, err, "second Init should return error")
}
