// Package main is the Tap binary entry point.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/poll"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/bcrisp4/tap/internal/server"
)

func main() {
	var (
		addr    = flag.String("addr", "127.0.0.1:8080", "HTTP listen address (set 0.0.0.0:8080 in containers)")
		dataDir = flag.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
		logFmt  = flag.String("log-format", "json", "log format: json or text")
	)
	flag.Parse()

	configureLogger(*logFmt)

	if err := os.MkdirAll(*dataDir, 0o755); err != nil {
		slog.Error("create data dir", "err", err)
		os.Exit(1)
	}
	dbPath := filepath.Join(*dataDir, "tap.db")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	d, err := db.Open(ctx, dbPath)
	if err != nil {
		slog.Error("open db", "path", dbPath, "err", err)
		os.Exit(1)
	}
	defer d.Close()

	if err := db.Migrate(ctx, d); err != nil {
		slog.Error("migrate", "err", err)
		os.Exit(1)
	}

	// HTTP client used by the polling worker pool. The 30s Client.Timeout
	// bounds total per-request time even when ctx isn't strictly enforced;
	// Transport timeouts cap connect / TLS / idle separately. Without these,
	// a slow origin would tie up a worker for the full 60s ctx ceiling.
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        32,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	sched.Start()

	mux := http.NewServeMux()
	// /api/ and /healthz both go through the same factory; sched.Poke is wired
	// into POST /api/v1/subscriptions so a freshly added feed polls immediately
	// rather than waiting up to TickInterval (60s).
	apiMux := api.NewMux(d, sched.Poke)
	mux.Handle("/api/", apiMux)
	mux.Handle("/healthz", apiMux)
	mux.Handle("/", server.SPAHandler())

	srv := server.New(server.Config{Addr: *addr, Handler: mux})
	if err := srv.Start(); err != nil {
		slog.Error("server start", "err", err)
		os.Exit(1)
	}

	<-ctx.Done()
	slog.Info("shutting down")

	// Stop accepting connections; drain in-flight HTTP. Then stop the scheduler
	// (which cancels in-flight worker ctxs and waits for them to finish).
	shutdownCtx, sCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer sCancel()
	_ = srv.Shutdown(shutdownCtx)
	sched.Stop()
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func configureLogger(format string) {
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(h))
}
