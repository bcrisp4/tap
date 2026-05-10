package metrics

import (
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

// Instrument variables — safe to call before Init (no-op instruments until Init is called).
// Each variable's description appears verbatim in the Prometheus HELP line.
var (
	// PollsTotal counts feed poll attempts by result.
	PollsTotal metric.Int64Counter = noop.Int64Counter{}

	// PollDuration measures wall-clock poll cycle duration in seconds.
	PollDuration metric.Float64Histogram = noop.Float64Histogram{}

	// EntriesInserted counts new entries committed to the database.
	EntriesInserted metric.Int64Counter = noop.Int64Counter{}

	// ConditionalGetHits counts polls receiving a 304 Not Modified response.
	ConditionalGetHits metric.Int64Counter = noop.Int64Counter{}

	// HTTPRequestsTotal counts inbound HTTP requests.
	HTTPRequestsTotal metric.Int64Counter = noop.Int64Counter{}

	// HTTPRequestDuration measures inbound request latency in seconds.
	HTTPRequestDuration metric.Float64Histogram = noop.Float64Histogram{}

	// ProxyCacheHits counts proxy requests served from filesystem cache.
	ProxyCacheHits metric.Int64Counter = noop.Int64Counter{}

	// ProxyCacheMisses counts proxy requests requiring an origin fetch.
	ProxyCacheMisses metric.Int64Counter = noop.Int64Counter{}

	// ProxyCacheEvictions counts evicted cache files by reason.
	ProxyCacheEvictions metric.Int64Counter = noop.Int64Counter{}

	// ProxyCacheBytes tracks total media cache size in bytes.
	ProxyCacheBytes metric.Int64Gauge = noop.Int64Gauge{}

	// LoginAttempts counts login attempts by outcome.
	LoginAttempts metric.Int64Counter = noop.Int64Counter{}

	// Lockouts counts lockout events triggered.
	Lockouts metric.Int64Counter = noop.Int64Counter{}

	// ActiveSessions tracks non-expired session rows (approximate).
	ActiveSessions metric.Int64Gauge = noop.Int64Gauge{}

	// DBTxDuration measures database transaction duration by operation.
	DBTxDuration metric.Float64Histogram = noop.Float64Histogram{}
)

func registerInstruments(mp *sdkmetric.MeterProvider) {
	m := mp.Meter("tap")

	PollsTotal, _ = m.Int64Counter("tap_polls_total",
		metric.WithDescription("Total feed poll attempts labelled by result (success, failure, skipped). "+
			"Use rate(tap_polls_total[5m]) to observe poll throughput."),
		metric.WithUnit("{polls}"))

	PollDuration, _ = m.Float64Histogram("tap_poll_duration_seconds",
		metric.WithDescription("Wall-clock duration of a complete poll cycle from dispatch to commit or failure, in seconds. "+
			"Buckets: 0.1s to 60s."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 5, 10, 30, 60))

	EntriesInserted, _ = m.Int64Counter("tap_entries_inserted_total",
		metric.WithDescription("Total new feed entries committed to the database across all polls. "+
			"Deduplicated entries (already seen) are not counted."),
		metric.WithUnit("{entries}"))

	ConditionalGetHits, _ = m.Int64Counter("tap_conditional_get_hits_total",
		metric.WithDescription("Number of polls that received HTTP 304 Not Modified, indicating the feed has not changed."),
		metric.WithUnit("{polls}"))

	HTTPRequestsTotal, _ = m.Int64Counter("tap_http_requests_total",
		metric.WithDescription("Total inbound HTTP requests labelled by method, matched route pattern (not raw path), "+
			"and response status class (2xx, 4xx, 5xx)."),
		metric.WithUnit("{requests}"))

	HTTPRequestDuration, _ = m.Float64Histogram("tap_http_request_duration_seconds",
		metric.WithDescription("Inbound HTTP request latency from first byte received to last byte written, in seconds. "+
			"Buckets: 5ms to 2.5s."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5))

	ProxyCacheHits, _ = m.Int64Counter("tap_proxy_cache_hits_total",
		metric.WithDescription("Media proxy requests served from the local filesystem cache without fetching the origin."),
		metric.WithUnit("{requests}"))

	ProxyCacheMisses, _ = m.Int64Counter("tap_proxy_cache_misses_total",
		metric.WithDescription("Media proxy requests that required fetching the origin (cache cold or expired)."),
		metric.WithUnit("{requests}"))

	ProxyCacheEvictions, _ = m.Int64Counter("tap_proxy_cache_evictions_total",
		metric.WithDescription("Media cache files evicted. reason=size_cap: inline LRU eviction. reason=age_sweep: daily archival sweep."),
		metric.WithUnit("{files}"))

	ProxyCacheBytes, _ = m.Int64Gauge("tap_proxy_cache_bytes",
		metric.WithDescription("Current total size of the media proxy filesystem cache in bytes. "+
			"Updated after each eviction pass and each new cache write."),
		metric.WithUnit("By"))

	LoginAttempts, _ = m.Int64Counter("tap_login_attempts_total",
		metric.WithDescription("Login attempts against POST /api/v1/sessions labelled by outcome. "+
			"result=rate_limited: per-source token bucket exhausted. result=locked_out: per-username lockout active."),
		metric.WithUnit("{attempts}"))

	Lockouts, _ = m.Int64Counter("tap_lockouts_total",
		metric.WithDescription("Lockout events triggered. axis=source: per-IP rate limit exceeded. "+
			"axis=username: per-username consecutive failure threshold reached."),
		metric.WithUnit("{lockouts}"))

	ActiveSessions, _ = m.Int64Gauge("tap_active_sessions",
		metric.WithDescription("Non-expired session rows in the database (approximate). "+
			"Updated on session create, delete, and expiry."),
		metric.WithUnit("{sessions}"))

	DBTxDuration, _ = m.Float64Histogram("tap_db_tx_duration_seconds",
		metric.WithDescription("Database transaction duration in seconds labelled by op "+
			"(e.g. insert_entries, list_due_polls, get_session). Buckets: 1ms to 1s."),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1))
}
