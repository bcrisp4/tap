package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peterbourgon/ff/v4"
	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/config"
)

// parse builds a fresh flag set, registers the config flags, and runs
// ff.Parse with the given args, env, and (optional) file. It returns
// the resolved Config.
func parse(t *testing.T, args []string, file string) *config.Config {
	t.Helper()
	fs := ff.NewFlagSet("test")
	var configPath string
	fs.StringVar(&configPath, 'c', "config", "", "config file")

	cfg := &config.Config{}
	config.RegisterFlags(fs, cfg)

	if file != "" {
		args = append([]string{"--config=" + file}, args...)
	}

	opts := []ff.Option{
		ff.WithEnvVarPrefix("TAP"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(config.YAMLParser),
		ff.WithConfigAllowMissingFile(),
	}
	require.NoError(t, ff.Parse(fs, args, opts...))
	return cfg
}

func TestRegisterFlags_Defaults(t *testing.T) {
	cfg := parse(t, nil, "")
	require.Equal(t, "./tap.db", cfg.DBPath)
	require.Equal(t, "127.0.0.1:8080", cfg.Listen)
	require.Equal(t, "info", cfg.LogLevel)
	require.Equal(t, "json", cfg.LogFormat)
	require.Equal(t, 4, cfg.PollWorkers)
	require.Equal(t, 60, cfg.ArchiveDays)
}

func TestRegisterFlags_FlagBeatsDefault(t *testing.T) {
	cfg := parse(t, []string{"--db-path=/tmp/x.db"}, "")
	require.Equal(t, "/tmp/x.db", cfg.DBPath)
}

func TestRegisterFlags_EnvBeatsDefault(t *testing.T) {
	t.Setenv("TAP_DB_PATH", "/env/x.db")
	t.Setenv("TAP_LOG_LEVEL", "debug")
	cfg := parse(t, nil, "")
	require.Equal(t, "/env/x.db", cfg.DBPath)
	require.Equal(t, "debug", cfg.LogLevel)
}

func TestRegisterFlags_FileBeatsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tap.yaml")
	require.NoError(t, os.WriteFile(path, []byte("db_path: /file/x.db\nlisten: 0.0.0.0:9000\n"), 0o644))

	cfg := parse(t, nil, path)
	require.Equal(t, "/file/x.db", cfg.DBPath)
	require.Equal(t, "0.0.0.0:9000", cfg.Listen)
}

func TestRegisterFlags_PrecedenceFlagOverEnvOverFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tap.yaml")
	require.NoError(t, os.WriteFile(path, []byte("db_path: /file/x.db\n"), 0o644))

	t.Setenv("TAP_DB_PATH", "/env/x.db")

	// env beats file
	cfg := parse(t, nil, path)
	require.Equal(t, "/env/x.db", cfg.DBPath, "env must beat file")

	// flag beats env beats file
	cfg = parse(t, []string{"--db-path=/flag/x.db"}, path)
	require.Equal(t, "/flag/x.db", cfg.DBPath, "flag must beat env")
}

func TestRegisterFlags_DurationParses(t *testing.T) {
	cfg := parse(t, []string{"--http-timeout=5s"}, "")
	require.Equal(t, "5s", cfg.HTTPTimeout.String())
}
