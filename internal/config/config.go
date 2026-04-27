// Package config defines the resolved Tap configuration and registers
// the matching ff/v4 flags.
package config

import (
	"time"

	"github.com/peterbourgon/ff/v4"
)

// Config holds every TAP_* knob from design.md §9.
//
// YAML keys match the flag long-name exactly (`--db-path` ⇄ `db-path`);
// ff/v4 resolves config-file keys against registered long-names with no
// translation needed. All `[]string` knobs accept either a repeated CLI
// flag (`--allowed-hosts=a --allowed-hosts=b`) or a YAML list
// (`allowed-hosts: [a, b]`).
type Config struct {
	DBPath    string
	Listen    string
	LogLevel  string
	LogFormat string

	PollInterval time.Duration
	PollWorkers  int
	PollFactor   float64
	HostWeight   int64

	UserAgent            string
	HTTPTimeout          time.Duration
	HTTPMaxBodyBytes     int64
	AllowPrivateNetworks bool
	AllowedHosts         []string
	IframeAllowlist      []string

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
// YAML key is the same long-name (db-path).
func RegisterFlags(fs *ff.FlagSet, cfg *Config) {
	fs.StringVar(&cfg.DBPath, 'd', "db-path", "./tap.db", "SQLite database file path")
	fs.StringVar(&cfg.Listen, 'l', "listen", "127.0.0.1:8080", "HTTP listen address")
	fs.StringVar(&cfg.LogLevel, 0, "log-level", "info", "Log level: debug | info | warn | error")
	fs.StringVar(&cfg.LogFormat, 0, "log-format", "json", "Log format: json | text")

	fs.DurationVar(&cfg.PollInterval, 0, "poll-interval", 60*time.Second, "Dispatcher tick interval")
	fs.IntVar(&cfg.PollWorkers, 0, "poll-workers", 4, "Worker pool size")
	fs.Float64Var(&cfg.PollFactor, 0, "poll-factor", 1.0, "Adaptive polling multiplier (lower = poll more often)")
	fs.Int64Var(&cfg.HostWeight, 0, "host-weight", 1, "Per-host concurrency cap shared by feed/article/proxy fetches")

	fs.StringVar(&cfg.UserAgent, 0, "user-agent",
		"Tap/0.1 (+https://github.com/bcrisp4/tap)",
		"Default outbound User-Agent")
	fs.DurationVar(&cfg.HTTPTimeout, 0, "http-timeout", 20*time.Second, "Outbound request timeout")
	fs.Int64Var(&cfg.HTTPMaxBodyBytes, 0, "http-max-body-bytes", 10*1024*1024, "Max response body for feed/article fetches")
	fs.BoolVarDefault(&cfg.AllowPrivateNetworks, 0, "allow-private-networks", false, "Disable the SSRF private-network block")
	fs.StringListVar(&cfg.AllowedHosts, 0, "allowed-hosts", "Host suffixes / CIDR blocks bypassing SSRF check (repeat flag or YAML list)")
	fs.StringListVar(&cfg.IframeAllowlist, 0, "iframe-allowlist", "Override iframe src host allowlist (repeat flag or YAML list)")

	fs.IntVar(&cfg.ArchiveDays, 0, "archive-days", 60, "Read & unsaved entries older than this are archived")

	fs.StringVar(&cfg.ProxyCacheDir, 0, "proxy-cache-dir", "./tap-cache/", "Filesystem cache directory for the media proxy")
	fs.Int64Var(&cfg.ProxyCacheMaxBytes, 0, "proxy-cache-max-bytes", 1024*1024*1024, "Max proxy cache size in bytes")
	fs.DurationVar(&cfg.ProxyCacheMaxAge, 0, "proxy-cache-max-age", 30*24*time.Hour, "Cache files older than this are deleted by the daily sweep")
	fs.DurationVar(&cfg.ProxyTimeout, 0, "proxy-timeout", 10*time.Second, "Origin-fetch timeout for proxy requests")
	fs.Int64Var(&cfg.ProxyMaxBodyBytes, 0, "proxy-max-body-bytes", 10*1024*1024, "Max body for proxy origin fetches")
}
