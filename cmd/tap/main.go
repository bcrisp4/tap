// Package main is the Tap binary entry point.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"flag"
	"fmt"
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
	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/bcrisp4/tap/internal/server"
)

func main() {
	var (
		addr          = flag.String("addr", "127.0.0.1:8080", "HTTP listen address (set 0.0.0.0:8080 in containers)")
		dataDir       = flag.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
		logFmt        = flag.String("log-format", "json", "log format: json or text")
		proxyCacheDir = flag.String("proxy-cache-dir", envOr("TAP_PROXY_CACHE_DIR", ""), "media cache directory (default: <data>/cache)")
		proxyCacheCap = flag.Int64("proxy-cache-cap-bytes", envOrInt64("TAP_PROXY_CACHE_CAP_BYTES", 524288000), "media cache size cap in bytes")
		proxyFetchTO  = flag.Duration("proxy-fetch-timeout", envOrDuration("TAP_PROXY_FETCH_TIMEOUT", 30*time.Second), "per-fetch deadline for media proxy origin requests")
		proxyBodyCap  = flag.Int64("proxy-body-cap-bytes", envOrInt64("TAP_PROXY_BODY_CAP_BYTES", 10485760), "per-response body cap for media proxy origin fetches")
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

	// Resolve proxy cache dir: explicit flag wins; otherwise <dataDir>/cache.
	cacheDir := *proxyCacheDir
	if cacheDir == "" {
		cacheDir = filepath.Join(*dataDir, "cache")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		slog.Error("create cache dir", "path", cacheDir, "err", err)
		os.Exit(1)
	}

	// Bootstrap the proxy signing key from the configuration table.
	proxyKey, err := loadOrCreateProxyKey(ctx, d)
	if err != nil {
		slog.Error("load proxy signing key", "err", err)
		os.Exit(1)
	}

	signer := proxy.NewSigner(proxyKey)
	cache := proxy.NewCache(cacheDir, *proxyCacheCap)

	// HTTP client used for both feed polls and proxy origin fetches.
	// M4 will replace this with a shared SSRF-aware client.
	client := &http.Client{
		Timeout: *proxyFetchTO,
		Transport: &http.Transport{
			MaxIdleConns:        32,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	proxyHandler := proxy.NewHandler(signer, cache, client, *proxyBodyCap)

	proc := processor.New(sanitise.DefaultPolicy(), signer.RewriteImageURL)

	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
		Processor: proc,
	})
	sched.Start()

	mux := http.NewServeMux()
	// /api/ and /healthz both go through the same factory; sched.Poke is wired
	// into POST /api/v1/subscriptions so a freshly added feed polls immediately
	// rather than waiting up to TickInterval (60s).
	apiMux := api.NewMux(d, sched.Poke, proxyHandler)
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

const proxySigningKeySize = 32

func loadOrCreateProxyKey(ctx context.Context, d *sql.DB) ([]byte, error) {
	if v, ok, err := db.GetConfig(ctx, d, "proxy.signing_key"); err != nil {
		return nil, fmt.Errorf("read signing key: %w", err)
	} else if ok && len(v) == proxySigningKeySize {
		return v, nil
	}

	key := make([]byte, proxySigningKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	got, err := db.SetConfigIfAbsent(ctx, d, "proxy.signing_key", key)
	if err != nil {
		return nil, err
	}
	return got, nil
}

func envOrInt64(k string, def int64) int64 {
	if v := os.Getenv(k); v != "" {
		var n int64
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
		slog.Warn("invalid int64 env var; falling back to default", "key", k, "value", v, "default", def)
	}
	return def
}

func envOrDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		slog.Warn("invalid duration env var; falling back to default", "key", k, "value", v, "default", def)
	}
	return def
}
