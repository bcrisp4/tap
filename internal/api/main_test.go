package api

import (
	"context"
	"os"
	"testing"

	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMain(m *testing.M) {
	reg := prometheus.NewRegistry()
	if err := metrics.InitWithRegistry(reg, metrics.Opts{}); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = metrics.Shutdown(context.Background())
	os.Exit(code)
}
