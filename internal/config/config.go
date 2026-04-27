// Package config defines the resolved Tap configuration and registers
// the matching ff/v4 flags.
package config

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffyaml"
)

// Config holds every TAP_* knob from design.md §9.
//
// YAML keys use the underscore form of the flag long-name (`--db-path` ⇄
// `db_path`); [YAMLParser] handles that translation. No struct tags
// required.
type Config struct {
	DBPath    string
	Listen    string
	LogLevel  string
	LogFormat string

	PollInterval time.Duration
	PollWorkers  int
	PollFactor   float64

	UserAgent            string
	HTTPTimeout          time.Duration
	HTTPMaxBodyBytes     int64
	AllowPrivateNetworks bool
	AllowedHosts         string
	IframeAllowlist      string

	ArchiveDays int

	ProxyCacheDir      string
	ProxyCacheMaxBytes int64
	ProxyCacheMaxAge   time.Duration
	ProxyTimeout       time.Duration
	ProxyMaxBodyBytes  int64
}

// RegisterFlags attaches every config knob as a flag on fs and binds
// the result to cfg's fields. The flag long-name is the lower-hyphen
// form of the field's TAP_* env-var name (TAP_DB_PATH ⇄ --db-path); the
// YAML key is the same name with underscores (db_path).
func RegisterFlags(fs *ff.FlagSet, cfg *Config) {
	fs.StringVar(&cfg.DBPath, 'd', "db-path", "./tap.db", "SQLite database file path")
	fs.StringVar(&cfg.Listen, 'l', "listen", "127.0.0.1:8080", "HTTP listen address")
	fs.StringVar(&cfg.LogLevel, 0, "log-level", "info", "Log level: debug | info | warn | error")
	fs.StringVar(&cfg.LogFormat, 0, "log-format", "json", "Log format: json | text")

	fs.DurationVar(&cfg.PollInterval, 0, "poll-interval", 60*time.Second, "Dispatcher tick interval")
	fs.IntVar(&cfg.PollWorkers, 0, "poll-workers", 4, "Worker pool size")
	fs.Float64Var(&cfg.PollFactor, 0, "poll-factor", 1.0, "Adaptive polling multiplier (lower = poll more often)")

	fs.StringVar(&cfg.UserAgent, 0, "user-agent",
		"Tap/0.1 (+https://github.com/bcrisp4/tap)",
		"Default outbound User-Agent")
	fs.DurationVar(&cfg.HTTPTimeout, 0, "http-timeout", 20*time.Second, "Outbound request timeout")
	fs.Int64Var(&cfg.HTTPMaxBodyBytes, 0, "http-max-body-bytes", 10*1024*1024, "Max response body for feed/article fetches")
	fs.BoolVarDefault(&cfg.AllowPrivateNetworks, 0, "allow-private-networks", false, "Disable the SSRF private-network block")
	fs.StringVar(&cfg.AllowedHosts, 0, "allowed-hosts", "", "Comma-separated host suffixes / CIDR blocks bypassing SSRF check")
	fs.StringVar(&cfg.IframeAllowlist, 0, "iframe-allowlist", "", "Override iframe src host allowlist (comma-separated)")

	fs.IntVar(&cfg.ArchiveDays, 0, "archive-days", 60, "Read & unsaved entries older than this are archived")

	fs.StringVar(&cfg.ProxyCacheDir, 0, "proxy-cache-dir", "./tap-cache/", "Filesystem cache directory for the media proxy")
	fs.Int64Var(&cfg.ProxyCacheMaxBytes, 0, "proxy-cache-max-bytes", 1024*1024*1024, "Max proxy cache size in bytes")
	fs.DurationVar(&cfg.ProxyCacheMaxAge, 0, "proxy-cache-max-age", 30*24*time.Hour, "Cache files older than this are deleted by the daily sweep")
	fs.DurationVar(&cfg.ProxyTimeout, 0, "proxy-timeout", 10*time.Second, "Origin-fetch timeout for proxy requests")
	fs.Int64Var(&cfg.ProxyMaxBodyBytes, 0, "proxy-max-body-bytes", 10*1024*1024, "Max body for proxy origin fetches")
}

// YAMLParser is a [ff.ConfigFileParseFunc] that wraps [ffyaml.Parse] and
// converts YAML keys from underscore form (db_path) to the hyphenated
// flag long-name form (--db-path) before resolving them. This matches
// design.md §9, where the YAML key is the same as the env-var name in
// lowercase (TAP_DB_PATH ⇄ db_path) while CLI flags use hyphens. Only
// the key is rewritten; values are forwarded verbatim, so underscores
// inside a value (e.g. a path) are preserved.
//
// YAML keys must be flat: top-level scalars, lists, or maps-of-scalars
// only. ffyaml stitches nested maps with `.` (so `proxy: { cache_dir:
// /x }` arrives here as `proxy.cache-dir` after rewriting), and Tap
// registers no flag for the dotted form, so nested maps surface as an
// "unknown flag" error from ff. Stick to flat keys matching the
// registered flag long-names with `-` ⇄ `_` translation.
//
// Tap registers only scalar flags, so YAML lists are not supported:
// ffyaml emits one set call per element and the second call would
// silently overwrite the first on a StringVar binding. To make this
// fail loudly, YAMLParser tracks which keys it has already forwarded
// and returns an error on the second occurrence, telling the user to
// use a comma-separated string for list-shaped values like
// allowed_hosts.
func YAMLParser(r io.Reader, set func(name, value string) error) error {
	seen := make(map[string]struct{})
	return ffyaml.Parse(r, func(name, value string) error {
		flag := strings.ReplaceAll(name, "_", "-")
		if _, dup := seen[flag]; dup {
			return fmt.Errorf("config: YAML key %q appears multiple times "+
				"(Tap config flags are scalar; for list-shaped values like "+
				"allowed_hosts use a comma-separated string)", name)
		}
		seen[flag] = struct{}{}
		return set(flag, value)
	})
}
