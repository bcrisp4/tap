// Command tap is the Tap feed reader binary.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/bcrisp4/tap/internal/config"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/log"
	"github.com/bcrisp4/tap/internal/server"
	"github.com/bcrisp4/tap/internal/version"
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
		ff.WithConfigFileParser(config.YAMLParser),
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

	srv, err := server.New(cfg.Listen, logger)
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}

	logger.Info("tap starting",
		"version", version.String(),
		"listen", cfg.Listen,
		"db_path", cfg.DBPath,
	)
	return srv.Run(ctx)
}
