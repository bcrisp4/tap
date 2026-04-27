// Command tap is the Tap feed reader binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

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

	var rootCmd *ff.Command
	serveCmd := &ff.Command{
		Name:      "serve",
		ShortHelp: "run the Tap HTTP server",
		Flags:     serveFS,
		Exec: func(ctx context.Context, _ []string) error {
			return runServe(ctx, cfg, stderr)
		},
	}
	rootCmd = &ff.Command{
		Name:        "tap",
		ShortHelp:   "self-hosted feed reader",
		Flags:       rootFS,
		Subcommands: []*ff.Command{serveCmd},
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
