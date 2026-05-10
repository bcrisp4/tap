// Package tracing owns the OTel TracerProvider for Tap.
// Call Init once at startup; the global provider is a no-op until then.
package tracing

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Opts configures the tracing provider.
type Opts struct {
	OTLPEndpoint string
	OTLPHeaders  map[string]string
	SampleRate   float64 // 0.0-1.0; default 0.1
	ServiceName  string
	Version      string
}

var (
	mu           sync.Mutex
	shutdownFunc func(context.Context) error
)

// Init initialises the OTel TracerProvider. When OTLPEndpoint is empty, a
// no-op sampler is used (spans created but not exported).
func Init(opts Opts) error {
	mu.Lock()
	defer mu.Unlock()

	rate := opts.SampleRate
	if rate <= 0 {
		rate = 0.1
	}

	sampler := sdktrace.ParentBased(
		sdktrace.TraceIDRatioBased(rate),
	)

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)
	shutdownFunc = tp.Shutdown
	return nil
}

// Shutdown flushes and closes the provider.
func Shutdown(ctx context.Context) error {
	mu.Lock()
	defer mu.Unlock()
	if shutdownFunc == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	err := shutdownFunc(ctx)
	shutdownFunc = nil
	return err
}
