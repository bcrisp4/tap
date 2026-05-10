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

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"golang.org/x/time/rate"

	"github.com/go-webauthn/webauthn/webauthn"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/archival"
	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpx"
	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/bcrisp4/tap/internal/poll"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/bcrisp4/tap/internal/ratelimit"
	"github.com/bcrisp4/tap/internal/ring"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/bcrisp4/tap/internal/server"
	"github.com/bcrisp4/tap/internal/tracing"
)

// version is set by the build via -ldflags "-X main.version=<tag>".
var version = "dev"

// main routes between the `tap admin ...` subcommand family and the regular
// server. Subcommands return an exit code so tests can call them directly.
func main() {
	if len(os.Args) >= 2 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck(os.Args[2:]))
	}
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

		archiveInterval = flag.Duration("archive-interval",
			envOrDuration("TAP_ARCHIVE_INTERVAL", 24*time.Hour),
			"how often the archival sweep runs")
		archiveHorizon = flag.Duration("archive-horizon",
			envOrDuration("TAP_ARCHIVE_HORIZON", 2160*time.Hour), // 90d
			"delete read+unsaved entries older than this horizon")
		cacheAgeCap = flag.Duration("cache-age-cap",
			envOrDuration("TAP_CACHE_AGE_CAP", 336*time.Hour), // 14d
			"unlink proxy cache files with fetched_at older than this age")

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

		// M12: observability + hardening flags.
		logLevel        = flag.String("log-level", envOr("TAP_LOG_LEVEL", "info"), "log level: debug, info, warn, error")
		metricsEnabled  = flag.Bool("metrics-enabled", envOrBool("TAP_METRICS_ENABLED", false), "enable GET /metrics Prometheus scrape endpoint on the main listener")
		otlpEndpoint    = flag.String("otlp-endpoint", envOr("TAP_OTLP_ENDPOINT", ""), "OTel collector endpoint (empty = disabled); http://, https://, or grpc:// scheme")
		otlpHeaders     = flag.String("otlp-headers", envOr("TAP_OTLP_HEADERS", ""), "comma-separated key=value OTLP auth headers")
		traceSampleRate = flag.Float64("trace-sample-rate", envOrFloat64("TAP_TRACE_SAMPLE_RATE", 0.1), "fraction of normal traces to sample (0.0-1.0)")

		loginRate        = flag.String("login-rate", envOr("TAP_LOGIN_RATE", "10/min"), "per-source login rate limit (N/min or N/s)")
		loginBurst       = flag.Int("login-burst", envOrInt("TAP_LOGIN_BURST", 5), "per-source burst allowance for login attempts")
		lockoutThreshold = flag.Int("lockout-threshold", envOrInt("TAP_LOCKOUT_THRESHOLD", 5), "consecutive per-username login failures before first lockout")
		lockoutBase      = flag.Duration("lockout-base", envOrDuration("TAP_LOCKOUT_BASE", 30*time.Second), "initial lockout duration")
		lockoutMax       = flag.Duration("lockout-max", envOrDuration("TAP_LOCKOUT_MAX", time.Hour), "maximum lockout duration after escalation")
		trustedProxy     = flag.Bool("trusted-proxy", envOrBool("TAP_TRUSTED_PROXY", false), "trust X-Forwarded-For for client IP in rate limiting and auth logs")

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

	configureLogger(*logFmt, *logLevel)

	startTime := time.Now()

	// Initialise metrics provider (Prometheus bridge + optional OTLP).
	if err := metrics.Init(metrics.Opts{
		OTLPEndpoint: *otlpEndpoint,
		OTLPHeaders:  parseOTLPHeaders(*otlpHeaders),
	}); err != nil {
		slog.Error("init metrics", "err", err)
		os.Exit(1)
	}

	// Wrap the slog default handler with the ring buffer so warn/error log
	// events are captured for the system-status panel.
	ringBuf := ring.NewBuffer(100)
	slog.SetDefault(slog.New(ring.NewHandler(slog.Default().Handler(), ringBuf)))

	// Initialise tracing provider (no-op when otlp-endpoint is empty).
	if err := tracing.Init(tracing.Opts{
		OTLPEndpoint: *otlpEndpoint,
		OTLPHeaders:  parseOTLPHeaders(*otlpHeaders),
		SampleRate:   *traceSampleRate,
		ServiceName:  "tap",
	}); err != nil {
		slog.Error("init tracing", "err", err)
		os.Exit(1)
	}

	slog.Info("tap starting", "event", "startup", "addr", *addr, "data_dir", *dataDir)

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

	archiver := archival.NewArchiver(d, archival.ArchiverOpts{
		Horizon:     *archiveHorizon,
		CacheAgeCap: *cacheAgeCap,
		Interval:    *archiveInterval,
		CacheDir:    cacheDir,
		OnEvict: func(n int) {
			metrics.ProxyCacheEvictions.Add(context.Background(), int64(n),
				metric.WithAttributes(attribute.String("reason", "age_sweep")))
		},
	})
	archiver.Start()

	limiter := ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      parseLoginRate(*loginRate),
		SourceBurst:     *loginBurst,
		FailThreshold:   *lockoutThreshold,
		LockoutBase:     *lockoutBase,
		LockoutMax:      *lockoutMax,
		CleanupInterval: 5 * time.Minute,
	})

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
		DiscoverClient:     client,
		Limiter:            limiter,
		TrustedProxy:       *trustedProxy,
		MetricsEnabled:     *metricsEnabled,
		RingBuffer:         ringBuf,
		StartTime:          startTime,
		Version:            version,
		PollsActive:        func() int64 { return sched.ActiveCount() },
	})
	mux.Handle("/api/", tracing.Middleware(apiMux))
	mux.Handle("/healthz", tracing.Middleware(apiMux))
	mux.Handle("/", server.SPAHandler())

	srv := server.New(server.Config{Addr: *addr, Handler: mux})
	if err := srv.Start(); err != nil {
		slog.Error("server start", "err", err)
		os.Exit(1)
	}

	<-ctx.Done()
	slog.Info("shutting down", "event", "shutdown", "reason", "signal")

	shutdownCtx, sCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer sCancel()
	_ = srv.Shutdown(shutdownCtx)
	archiver.Stop()
	sched.Stop()
	limiter.Stop()
	client.CloseIdleConnections()
	_ = tracing.Shutdown(shutdownCtx)
	_ = metrics.Shutdown(shutdownCtx)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func configureLogger(format, level string) {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
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

func envOrFloat64(k string, def float64) float64 {
	if v := os.Getenv(k); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid float64; using default %g\n", k, v, def)
	}
	return def
}

func parseOTLPHeaders(raw string) map[string]string {
	m := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		m[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return m
}

func parseLoginRate(s string) rate.Limit {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "/min") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "/min"))
		if err == nil && n > 0 {
			return rate.Every(time.Minute / time.Duration(n))
		}
	}
	if strings.HasSuffix(s, "/s") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "/s"))
		if err == nil && n > 0 {
			return rate.Limit(n)
		}
	}
	return rate.Every(6 * time.Second) // default 10/min
}

func runHealthcheck(args []string) int {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	addr := fs.String("addr", envOr("TAP_ADDR", "127.0.0.1:8080"), "server address to check")
	timeout := fs.Duration("timeout", 5*time.Second, "HTTP request timeout")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	host, port, err := net.SplitHostPort(*addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "invalid addr:", err)
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	client := &http.Client{Timeout: *timeout}
	url := "http://" + net.JoinHostPort(host, port) + "/healthz"
	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck failed:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return 0
	}
	fmt.Fprintln(os.Stderr, "healthcheck: unexpected status", resp.StatusCode)
	return 1
}

// stringSlice implements flag.Value for repeatable string flags.
// Env-var form is comma-separated; flag form is repeatable.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}
