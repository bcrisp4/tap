package proxy_test

import (
	"context"
	"os"
	"testing"

	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

// sharedReg is the Prometheus registry used for all proxy metrics tests.
var sharedReg *prometheus.Registry

func TestMain(m *testing.M) {
	sharedReg = prometheus.NewRegistry()
	if err := metrics.InitWithRegistry(sharedReg, metrics.Opts{}); err != nil {
		panic(err)
	}
	code := m.Run()
	_ = metrics.Shutdown(context.Background())
	os.Exit(code)
}
