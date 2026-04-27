// Command tap is the Tap feed reader binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"github.com/peterbourgon/ff/v4/ffyaml"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/config"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/log"
	"github.com/bcrisp4/tap/internal/poller"
	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/bcrisp4/tap/internal/reader"
	"github.com/bcrisp4/tap/internal/server"
	"github.com/bcrisp4/tap/internal/storage"
	"github.com/bcrisp4/tap/internal/version"
	"github.com/bcrisp4/tap/internal/web"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	os.Exit(run(ctx, os.Args, os.Stdout, os.Stderr))
}

// run is the testable entrypoint. It parses args (with env + YAML file
// merged in via ff/v4) and dispatches to the matching subcommand.
func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	var (
		showVersion bool
		configPath  string
	)

	rootFS := ff.NewFlagSet("tap")
	rootFS.BoolVar(&showVersion, 'v', "version", "Print version and exit")
	rootFS.StringVar(&configPath, 'c', "config", "", "Path to YAML config file (or set TAP_CONFIG)")

	cfg := &config.Config{}
	serveFS := ff.NewFlagSet("serve").SetParent(rootFS)
	config.RegisterFlags(serveFS, cfg)

	var healthcheckURL string
	healthcheckFS := ff.NewFlagSet("healthcheck").SetParent(rootFS)
	healthcheckFS.StringVar(&healthcheckURL, 'u', "url",
		"http://127.0.0.1:8080/healthz",
		"URL to probe for the healthcheck (also TAP_URL)")

	var rootCmd *ff.Command
	serveCmd := &ff.Command{
		Name:      "serve",
		ShortHelp: "run the Tap HTTP server",
		Flags:     serveFS,
		Exec: func(ctx context.Context, _ []string) error {
			return runServe(ctx, cfg, stderr)
		},
	}
	healthcheckCmd := &ff.Command{
		Name:      "healthcheck",
		ShortHelp: "probe a running tap server (for Docker HEALTHCHECK)",
		Flags:     healthcheckFS,
		Exec: func(ctx context.Context, _ []string) error {
			if err := healthcheckProbe(ctx, healthcheckURL); err != nil {
				return fmt.Errorf("healthcheck: %w", err)
			}
			return nil
		},
	}
	rootCmd = &ff.Command{
		Name:        "tap",
		ShortHelp:   "self-hosted feed reader",
		Flags:       rootFS,
		Subcommands: []*ff.Command{serveCmd, healthcheckCmd},
		Exec: func(_ context.Context, args []string) error {
			if showVersion {
				fmt.Fprintf(stdout, "tap %s\n", version.String())
				return nil
			}
			if len(args) > 0 {
				return fmt.Errorf("unknown subcommand %q", args[0])
			}
			fmt.Fprint(stderr, ffhelp.Command(rootCmd))
			return nil
		},
	}

	fail := func(err error) { fmt.Fprintf(stderr, "tap: %v\n", err) }

	parseErr := rootCmd.Parse(args[1:],
		ff.WithEnvVarPrefix("TAP"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ffyaml.Parse),
		ff.WithConfigAllowMissingFile(),
	)
	switch {
	case errors.Is(parseErr, ff.ErrHelp):
		fmt.Fprint(stderr, ffhelp.Command(rootCmd))
		return 0
	case parseErr != nil:
		fail(parseErr)
		return 2
	}

	if err := rootCmd.Run(ctx); err != nil {
		fail(err)
		return 1
	}
	return 0
}

// healthcheckProbe is the function the `tap healthcheck` subcommand
// uses to verify a running server. It's a package-level var so tests
// can stub it out without standing up a real HTTP server.
var healthcheckProbe = defaultHealthcheckProbe

// defaultHealthcheckProbe issues a single GET against url with a 5 s
// timeout. Any non-2xx response or transport error returns a non-nil
// error so the subcommand exits non-zero — Docker's HEALTHCHECK
// directive treats that as "unhealthy".
func defaultHealthcheckProbe(ctx context.Context, url string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", url, err)
	}
	// Drain before close so the underlying TCP/keep-alive connection can
	// be reused; matters when the same probe runs repeatedly under
	// Docker's HEALTHCHECK loop.
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("probe %s: status %d", url, resp.StatusCode)
	}
	return nil
}

func runServe(ctx context.Context, cfg *config.Config, stderr io.Writer) error {
	logger, err := log.New(cfg.LogLevel, cfg.LogFormat, stderr)
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	sqliteDB, err := db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqliteDB.Close()

	if err := db.Migrate(ctx, sqliteDB); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	store := storage.New(sqliteDB)

	allow, err := httpclient.ParseAllowedHosts(strings.Join(cfg.AllowedHosts, ","))
	if err != nil {
		return fmt.Errorf("parse allowed hosts: %w", err)
	}
	httpCli := httpclient.NewClient(httpclient.Config{
		Timeout:      cfg.HTTPTimeout,
		MaxBodyBytes: cfg.HTTPMaxBodyBytes,
		AllowPrivate: cfg.AllowPrivateNetworks,
		Allowlist:    allow,
		UserAgent:    cfg.UserAgent,
	})

	secret, err := proxy.EnsureSecret(ctx, store)
	if err != nil {
		return fmt.Errorf("ensure proxy secret: %w", err)
	}
	prox := proxy.New(proxy.Config{
		Secret:        secret,
		Client:        httpCli,
		Cache:         proxy.NewCache(cfg.ProxyCacheDir),
		MaxBodyBytes:  cfg.ProxyMaxBodyBytes,
		MaxCacheBytes: cfg.ProxyCacheMaxBytes,
	})

	pipeline := reader.NewPipeline(reader.PipelineConfig{
		Encode:          prox.Encoder(),
		IframeAllowlist: cfg.IframeAllowlist,
	})

	pol := poller.New(poller.Config{
		Store:            store,
		Client:           httpCli,
		Pipeline:         pipeline,
		Interval:         cfg.PollInterval,
		Workers:          cfg.PollWorkers,
		PollFactor:       cfg.PollFactor,
		ArchiveDays:      cfg.ArchiveDays,
		ProxyCacheDir:    cfg.ProxyCacheDir,
		ProxyCacheMaxAge: cfg.ProxyCacheMaxAge,
		HostWeight:       cfg.HostWeight,
	})
	// Plan 07 builds and owns the per-host limiter inside the poller;
	// when the media proxy is retrofitted to take one (design §6
	// follow-up), pass `pol.HostLimiter()` into proxy.Config.

	srv, err := server.New(cfg.Listen, logger)
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}
	srv.Mount("/api/v1/proxy/", prox)
	// api.Mux registers full /api/v1/* patterns internally, so we
	// mount it at the root: ServeMux's longest-prefix rule lets the
	// proxy mount at /api/v1/proxy/ keep precedence over the API
	// mux's GET/POST/... patterns at /api/v1/*.
	srv.Mount("/api/v1/", api.Mux(api.Dependencies{
		Store:      store,
		HTTPClient: httpCli,
		RunState:   pol.State(),
	}))
	// SPA fallback: every non-API path falls through to the embedded
	// SvelteKit build (or the placeholder when the SPA isn't embedded).
	// http.ServeMux's longest-prefix-wins routing keeps /api/v1/* and
	// /healthz registered above this from matching here.
	srv.Mount("/", web.Handler())

	logger.Info("tap starting",
		"version", version.String(),
		"listen", cfg.Listen,
		"db_path", cfg.DBPath,
		"workers", cfg.PollWorkers,
	)

	go pol.Start(ctx)
	return srv.Run(ctx)
}
