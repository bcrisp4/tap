package poller

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAdaptive_RetryAfterWins(t *testing.T) {
	got := NextPollAt(PollOutcome{
		Now: 1000, Failed: false, RetryAfter: 30 * time.Minute,
		ErrorCount:    5, // would be reset
		WeeklyEntries: 100,
	}, 1.0)
	require.Equal(t, int64(1000+1800), got.NextPollAt)
	require.Equal(t, 0, got.ErrorCount, "Retry-After resets error count")
}

func TestAdaptive_FailureExponentialBackoff(t *testing.T) {
	got := NextPollAt(PollOutcome{
		Now: 1000, Failed: true, ErrorCount: 2,
	}, 1.0)
	// 2^3 = 8h
	require.Equal(t, int64(1000+int64(8*time.Hour/time.Second)), got.NextPollAt)
	require.Equal(t, 3, got.ErrorCount)
}

func TestAdaptive_FailureCappedAt24h(t *testing.T) {
	got := NextPollAt(PollOutcome{Now: 0, Failed: true, ErrorCount: 20}, 1.0)
	require.Equal(t, int64(24*3600), got.NextPollAt)
}

func TestAdaptive_CacheControlIsFloor(t *testing.T) {
	// Adaptive would say 15m (very busy feed); max-age=2h overrides.
	out := NextPollAt(PollOutcome{
		Now: 1000, WeeklyEntries: 1000, MaxAge: 2 * time.Hour,
	}, 1.0)
	require.Equal(t, int64(1000+int64(2*time.Hour/time.Second)), out.NextPollAt)
}

func TestAdaptive_QuietFeed24h(t *testing.T) {
	got := NextPollAt(PollOutcome{Now: 1000, WeeklyEntries: 0}, 1.0)
	require.Equal(t, int64(1000+24*3600), got.NextPollAt)
}

func TestAdaptive_BusyFeedBoundedAt15m(t *testing.T) {
	got := NextPollAt(PollOutcome{Now: 1000, WeeklyEntries: 100000}, 1.0)
	require.Equal(t, int64(1000+15*60), got.NextPollAt)
}

func TestAdaptive_FactorScales(t *testing.T) {
	// 1 entry/wk → 168h, way over the 24h ceiling, so clamped to 24h.
	out := NextPollAt(PollOutcome{Now: 0, WeeklyEntries: 1}, 1.0)
	require.Equal(t, int64(24*3600), out.NextPollAt)
	// Factor=2 (poll less often) — still clamped to 24h.
	out = NextPollAt(PollOutcome{Now: 0, WeeklyEntries: 1}, 2.0)
	require.Equal(t, int64(24*3600), out.NextPollAt)
	// 200 entries/wk → 168/200 = 0.84h ≈ 50m.
	out = NextPollAt(PollOutcome{Now: 0, WeeklyEntries: 200}, 1.0)
	require.InDelta(t, 168.0/200.0*3600, float64(out.NextPollAt), 5)
	// Factor=2 → 0.42h ≈ 25m, clamped above 15m.
	out = NextPollAt(PollOutcome{Now: 0, WeeklyEntries: 200}, 2.0)
	require.InDelta(t, 168.0/200.0/2.0*3600, float64(out.NextPollAt), 5)
}
