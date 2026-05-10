// Package metrics owns the OTel MeterProvider and Prometheus exporter for Tap.
// Call Init once at startup; the global provider is a no-op until then.
// All instrument variables in instruments.go are safe to call before Init.
package metrics

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

// Opts configures the metrics provider.
type Opts struct {
	OTLPEndpoint string
	OTLPHeaders  map[string]string
}

var (
	mu           sync.Mutex
	shutdownFunc func(context.Context) error
	globalReg    = prometheus.NewRegistry()
)

// Init initialises the OTel MeterProvider. Returns an error if called twice.
func Init(opts Opts) error {
	return InitWithRegistry(globalReg, opts)
}

// InitWithRegistry allows tests to supply an isolated registry.
func InitWithRegistry(reg *prometheus.Registry, opts Opts) error {
	mu.Lock()
	defer mu.Unlock()
	if shutdownFunc != nil {
		return errors.New("metrics: already initialised")
	}

	exp, err := promexporter.New(
		promexporter.WithRegisterer(reg),
		promexporter.WithoutTargetInfo(),
		promexporter.WithoutScopeInfo(),
	)
	if err != nil {
		return err
	}

	mp := metric.NewMeterProvider(metric.WithReader(exp))

	shutdownFunc = func(ctx context.Context) error {
		err := mp.Shutdown(ctx)
		// Reset global registry for potential re-init in tests.
		mu.Lock()
		globalReg = prometheus.NewRegistry()
		shutdownFunc = nil
		mu.Unlock()
		return err
	}
	registerInstruments(mp)

	// Store the registry so Handler() can use it.
	activeReg = reg
	return nil
}

// Shutdown flushes and closes the provider.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	fn := shutdownFunc
	mu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

var activeReg = globalReg

// Handler returns an HTTP handler serving Prometheus text exposition.
// Only mount when --metrics-enabled.
func Handler() http.Handler {
	return promhttp.HandlerFor(activeReg, promhttp.HandlerOpts{EnableOpenMetrics: false})
}
