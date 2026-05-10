// Package main is the Tap binary entry point.
package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpx"
	"github.com/bcrisp4/tap/internal/poll"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/bcrisp4/tap/internal/server"
)

// main routes between the `tap admin ...` subcommand family and the regular
// server. Subcommands return an exit code so tests can call them directly.
func main() {
	if len(os.Args) >= 2 && os.Args[1] == "admin" {
		os.Exit(runAdmin(os.Args[2:], os.Stdin, os.Stdout, os.Stderr, auth.DefaultParams))
	}
	runServer()
}

// runServer is the long-running HTTP + scheduler entry point. This is the
// historical body of main(); it became its own function when the admin
// subcommand dispatcher landed.
func runServer() {
	var (
		addr    = flag.String("addr", "127.0.0.1:8080", "HTTP listen address (set 0.0.0.0:8080 in containers)")
		dataDir = flag.String("data", envOr("TAP_DATA_DIR", "./data"), "data directory containing tap.db")
		logFmt  = flag.String("log-format", "json", "log format: json or text")

		httpTimeout   = flag.Duration("http-timeout", envOrDuration("TAP_HTTP_TIMEOUT", 30*time.Second), "total per-request HTTP deadline")
		perHostInfl   = flag.Int("per-host-inflight", envOrInt("TAP_PER_HOST_INFLIGHT", 4), "concurrent outbound HTTP requests per hostname")
		ssrfDisabled  = flag.Bool("ssrf-disabled", envOrBool("TAP_SSRF_DISABLED", false), "disable the SSRF guard (use only on fully trusted networks)")
		pollFloor     = flag.Duration("poll-floor", envOrDuration("TAP_POLL_FLOOR", 15*time.Minute), "adaptive cadence floor")
		pollCeiling   = flag.Duration("poll-ceiling", envOrDuration("TAP_POLL_CEILING", 24*time.Hour), "adaptive cadence ceiling and error backoff cap")
		pollErrorBase = flag.Duration("poll-error-base", envOrDuration("TAP_POLL_ERROR_BASE", 5*time.Minute), "base of exponential error backoff")

		extractConcurrency = flag.Int("extract-concurrency", envOrInt("TAP_EXTRACT_CONCURRENCY", 4),
			"per-worker parallel article fetches when a subscription has extract=true")
		extractBodyCap = flag.Int64("extract-body-cap-bytes", envOrInt64("TAP_EXTRACT_BODY_CAP_BYTES", 5<<20),
			"per-article HTTP body cap before extraction parses it")

		userAgent = flag.String("user-agent", envOr("TAP_USER_AGENT", "tap/0.1 (+https://github.com/bcrisp4/tap)"), "User-Agent header on outbound HTTP")

		proxyCacheDir = flag.String("proxy-cache-dir", envOr("TAP_PROXY_CACHE_DIR", ""), "media cache directory (default: <data>/cache)")
		proxyCacheCap = flag.Int64("proxy-cache-cap-bytes", envOrInt64("TAP_PROXY_CACHE_CAP_BYTES", 524288000), "media cache size cap in bytes")
		proxyBodyCap  = flag.Int64("proxy-body-cap-bytes", envOrInt64("TAP_PROXY_BODY_CAP_BYTES", 10485760), "per-response body cap for media proxy origin fetches")

		// Auth-related flags. The defaults track concept §7.4: 7-day idle TTL
		// (refreshed on activity), 90-day absolute cap. Cookie Secure default
		// is "auto": derived from --addr (loopback => off, anything else => on).
		sessionIdleTTL = flag.Duration("session-idle-ttl",
			envOrDuration("TAP_SESSION_IDLE_TTL", 7*24*time.Hour),
			"refresh-on-activity expiry for session cookies")
		sessionAbsoluteTTL = flag.Duration("session-absolute-ttl",
			envOrDuration("TAP_SESSION_ABSOLUTE_TTL", 90*24*time.Hour),
			"hard cap on session lifetime regardless of activity")
		cookieSecureMode = flag.String("cookie-secure",
			envOr("TAP_COOKIE_SECURE", "auto"),
			"set Secure attribute on session cookie: auto|true|false")

		webAuthnRPID   = flag.String("webauthn-rp-id", envOr("TAP_WEBAUTHN_RP_ID", ""), "WebAuthn relying party ID (hostname); derived from --addr if empty")
		webAuthnOrigin = flag.String("webauthn-origin", envOr("TAP_WEBAUTHN_ORIGIN", ""), "WebAuthn origin URL; derived from --addr if empty")

		ssrfAllow stringSlice
	)
	flag.Var(&ssrfAllow, "ssrf-allow", "SSRF allowlist entry (CIDR, IP literal, or hostname suffix). Repeatable; env TAP_SSRF_ALLOW is comma-separated.")
	flag.Parse()

	// Hydrate ssrfAllow from env if not set via flag.
	if len(ssrfAllow) == 0 {
		if env := os.Getenv("TAP_SSRF_ALLOW"); env != "" {
			for _, p := range strings.Split(env, ",") {
				if p = strings.TrimSpace(p); p != "" {
					ssrfAllow = append(ssrfAllow, p)
				}
			}
		}
	}

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

	// First-launch admin bootstrap. Concept §7.1: when the DB has no users
	// AND the operator supplied TAP_ADMIN_USERNAME + TAP_ADMIN_PASSWORD,
	// create an admin row from those env vars. If users already exist, this
	// path is a silent no-op even when the env vars are still set — that's
	// the contract that lets containers keep the env vars in their
	// definition without re-bootstrapping on every restart.
	if n, err := db.CountUsers(ctx, d); err != nil {
		slog.Error("count users", "err", err)
		os.Exit(1)
	} else if n == 0 {
		user := os.Getenv("TAP_ADMIN_USERNAME")
		pass := os.Getenv("TAP_ADMIN_PASSWORD")
		switch {
		case user != "" && pass != "":
			if err := bootstrapAdmin(ctx, d, user, pass, auth.DefaultParams); err != nil {
				slog.Error("bootstrap admin", "err", err)
				os.Exit(1)
			}
			slog.Info("bootstrapped admin from environment", "username", user)
		case user != "" || pass != "":
			slog.Error("partial admin bootstrap: both TAP_ADMIN_USERNAME and TAP_ADMIN_PASSWORD must be set")
			os.Exit(1)
		default:
			slog.Warn("no users in database; create one with 'tap admin create' or set TAP_ADMIN_USERNAME and TAP_ADMIN_PASSWORD")
		}
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

	// Build the shared SSRF-aware HTTP client. The same client serves both
	// feed polls (via the scheduler) and proxy origin fetches, so the per-host
	// concurrency cap, SSRF guard, and User-Agent apply uniformly.
	ssrfPolicy, err := httpx.ParseSSRFPolicy(*ssrfDisabled, []string(ssrfAllow))
	if err != nil {
		slog.Error("parse ssrf-allow", "err", err)
		os.Exit(1)
	}
	if *ssrfDisabled {
		slog.Warn("SSRF guard disabled — outbound HTTP unrestricted")
	}

	client := httpx.NewClient(httpx.Opts{
		Timeout:         *httpTimeout,
		PerHostInflight: *perHostInfl,
		SSRF:            ssrfPolicy,
		UserAgent:       *userAgent,
	})

	proxyHandler := proxy.NewHandler(signer, cache, client, *proxyBodyCap)

	proc := processor.New(sanitise.DefaultPolicy(), signer.RewriteImageURL)

	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
		Processor:          proc,
		Floor:              *pollFloor,
		Ceiling:            *pollCeiling,
		ErrorBase:          *pollErrorBase,
		ExtractConcurrency: *extractConcurrency,
		ExtractBodyCap:     *extractBodyCap,
	})
	sched.Start()

	// Resolve --cookie-secure once at startup. "auto" (the default) inspects
	// --addr: loopback bind => Secure off, anything else => Secure on. Any
	// other token is a startup error.
	var cookieSecureEnum api.CookieSecureMode
	switch strings.ToLower(*cookieSecureMode) {
	case "auto", "":
		cookieSecureEnum = api.CookieSecureAuto
	case "true":
		cookieSecureEnum = api.CookieSecureTrue
	case "false":
		cookieSecureEnum = api.CookieSecureFalse
	default:
		slog.Error("invalid --cookie-secure", "value", *cookieSecureMode)
		os.Exit(1)
	}
	cookieSecure := api.ResolveCookieSecure(cookieSecureEnum, *addr)

	// Derive WebAuthn RPID and origin from --addr if not set explicitly.
	waRPID := *webAuthnRPID
	waOrigin := *webAuthnOrigin
	if waRPID == "" || waOrigin == "" {
		host, port, _ := net.SplitHostPort(*addr)
		if host == "" {
			host = "localhost"
		}
		// Normalise any loopback IP (127.x.x.x, ::1) to "localhost" so that
		// the WebAuthn RP ID matches the browser's origin when the user opens
		// the app at http://localhost:<port>. Without this, rpID would be
		// "127.0.0.1" but the browser presents the origin as "localhost".
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			host = "localhost"
		}
		if waRPID == "" {
			waRPID = host
		}
		if waOrigin == "" {
			scheme := "http"
			if cookieSecure {
				scheme = "https"
			}
			// Use waRPID (already normalised) rather than the raw addr so that
			// the origin and RPID are consistent (both "localhost", not a mix
			// of "localhost" and "127.0.0.1").
			waOrigin = scheme + "://" + waRPID + ":" + port
		}
	}
	var waInstance *webauthn.WebAuthn
	if wai, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "Tap",
		RPID:          waRPID,
		RPOrigins:     []string{waOrigin},
	}); err != nil {
		slog.Error("webauthn init", "err", err)
		os.Exit(1)
	} else {
		waInstance = wai
	}

	mux := http.NewServeMux()
	apiMux := api.NewMux(d, api.MuxOpts{
		Poke:               sched.Poke,
		ProxyHandler:       proxyHandler,
		SessionIdleTTL:     *sessionIdleTTL,
		SessionAbsoluteTTL: *sessionAbsoluteTTL,
		CookieSecure:       cookieSecure,
		HashParams:         auth.DefaultParams,
		WebAuthnInstance:   waInstance,
	})
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
	client.CloseIdleConnections()
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

// proxySigningKeyConfigKey is the row in the configuration table that holds
// the HMAC signing key for proxy URLs. Constant so the bootstrap path and
// tests can't drift on a typo.
const proxySigningKeyConfigKey = "proxy.signing_key"

func loadOrCreateProxyKey(ctx context.Context, d *sql.DB) ([]byte, error) {
	// A stored key takes precedence and is returned as-is. proxy.NewSigner
	// will panic with a clear message if the stored key is shorter than
	// proxy.KeySize — that's the loud-failure path for an operator who
	// somehow ended up with a malformed row. Per spec, key rotation is not
	// a feature, so we never overwrite a stored key from this code path.
	if v, ok, err := db.GetConfig(ctx, d, proxySigningKeyConfigKey); err != nil {
		return nil, fmt.Errorf("read signing key: %w", err)
	} else if ok {
		return v, nil
	}

	key := make([]byte, proxy.KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate signing key: %w", err)
	}
	got, err := db.SetConfigIfAbsent(ctx, d, proxySigningKeyConfigKey, key)
	if err != nil {
		return nil, fmt.Errorf("persist signing key: %w", err)
	}
	return got, nil
}

// envOr* helpers are evaluated during flag-default evaluation, which runs
// before configureLogger. We can't use slog here — it'd write to the
// default text handler regardless of the operator's --log-format choice.
// Plain stderr is the right channel for a startup configuration warning.

func envOrInt64(k string, def int64) int64 {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
		fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid int64; using default %d\n", k, v, def)
	}
	return def
}

func envOrDuration(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid duration; using default %s\n", k, v, def)
	}
	return def
}

func envOrInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
		fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid int; using default %d\n", k, v, def)
	}
	return def
}

func envOrBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
		fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid bool; using default %v\n", k, v, def)
	}
	return def
}

// stringSlice implements flag.Value for repeatable string flags.
// Env-var form is comma-separated; flag form is repeatable.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}
