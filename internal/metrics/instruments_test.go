package metrics_test

import (
	"context"
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstruments_HelpStringsPresent(t *testing.T) {
	reg := prometheus.NewRegistry()
	err := metrics.InitWithRegistry(reg, metrics.Opts{})
	require.NoError(t, err)
	defer metrics.Shutdown(context.Background())

	// Record a value so the metric appears in output.
	metrics.PollsTotal.Add(context.Background(), 1)

	gathered, err := reg.Gather()
	require.NoError(t, err)

	var found bool
	for _, mf := range gathered {
		if mf.GetName() == "tap_polls_total" {
			assert.True(t, strings.Contains(mf.GetHelp(), "poll"), "help string should mention poll")
			found = true
		}
	}
	assert.True(t, found, "tap_polls_total should appear in gathered metrics")
}
