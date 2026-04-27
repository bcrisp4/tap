package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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

// captureSet returns a set callback that records every (name, value)
// pair YAMLParser hands it, plus a pointer to that record slice.
type kv struct{ name, value string }

func captureSet() (func(name, value string) error, *[]kv) {
	var calls []kv
	return func(name, value string) error {
		calls = append(calls, kv{name, value})
		return nil
	}, &calls
}

func TestYAMLParser_HyphenatesUnderscoredKeys(t *testing.T) {
	set, calls := captureSet()
	require.NoError(t, config.YAMLParser(strings.NewReader("db_path: /foo\n"), set))
	require.Equal(t, []kv{{"db-path", "/foo"}}, *calls)
}

func TestYAMLParser_PreservesUnderscoresInValues(t *testing.T) {
	set, calls := captureSet()
	require.NoError(t, config.YAMLParser(strings.NewReader("db_path: /path_with_underscore\n"), set))
	require.Equal(t, []kv{{"db-path", "/path_with_underscore"}}, *calls)
}

func TestYAMLParser_NestedMapsStitchWithDot(t *testing.T) {
	// ffyaml flattens nested maps by stitching segments with `.`. The
	// rewriter only touches `_`, so a nested map surfaces here as
	// `proxy.cache-dir`. (ff would reject this as an unknown flag in
	// real use; the parser itself faithfully forwards the dotted form.)
	set, calls := captureSet()
	yaml := "proxy:\n  cache_dir: /x\n"
	require.NoError(t, config.YAMLParser(strings.NewReader(yaml), set))
	require.Equal(t, []kv{{"proxy.cache-dir", "/x"}}, *calls)
}

func TestYAMLParser_ListErrors(t *testing.T) {
	// Tap registers only scalar flags. ffyaml emits one set-call per
	// list element, so without protection a YAML list silently collapses
	// to its last element. YAMLParser must instead fail loudly on the
	// duplicate key with guidance to use a comma-separated string.
	set, calls := captureSet()
	yaml := "allowed_hosts:\n  - a\n  - b\n"
	err := config.YAMLParser(strings.NewReader(yaml), set)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"allowed_hosts"`)
	require.Contains(t, err.Error(), "comma-separated")
	// Only the first element reached set; the duplicate aborted parsing.
	require.Equal(t, []kv{{"allowed-hosts", "a"}}, *calls)
}

func TestYAMLParser_ListErrorsThroughFFParse(t *testing.T) {
	// End-to-end: a YAML config file containing a list for a scalar
	// flag must abort ff.Parse instead of silently keeping the last
	// element.
	dir := t.TempDir()
	path := filepath.Join(dir, "tap.yaml")
	require.NoError(t, os.WriteFile(path, []byte("allowed_hosts:\n  - a\n  - b\n"), 0o644))

	fs := ff.NewFlagSet("test")
	var configPath string
	fs.StringVar(&configPath, 'c', "config", "", "config file")
	cfg := &config.Config{}
	config.RegisterFlags(fs, cfg)

	err := ff.Parse(fs, []string{"--config=" + path},
		ff.WithEnvVarPrefix("TAP"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(config.YAMLParser),
		ff.WithConfigAllowMissingFile(),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"allowed_hosts"`)
}

func TestYAMLParser_PropagatesSetError(t *testing.T) {
	boom := errors.New("boom")
	err := config.YAMLParser(
		strings.NewReader("db_path: /foo\n"),
		func(name, value string) error { return boom },
	)
	require.ErrorIs(t, err, boom)
}
