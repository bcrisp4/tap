package cadence

import (
	"math/rand/v2"
	"testing"
	"time"
)

func TestIntervalFromVelocity(t *testing.T) {
	floor := 15 * time.Minute
	ceiling := 24 * time.Hour
	cases := []struct {
		name         string
		velocityX100 int
		want         time.Duration
	}{
		{"zero velocity hits ceiling", 0, ceiling},
		{"half entry per day hits ceiling", 50, ceiling},
		{"one entry per day exactly hits ceiling", 100, ceiling},
		{"two entries per day -> 12h", 200, 12 * time.Hour},
		{"ten entries per day -> 2h24m", 1000, 2*time.Hour + 24*time.Minute},
		{"96 entries per day -> 15m floor", 9600, floor},
		{"200 entries per day clamped to floor", 20000, floor},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IntervalFromVelocity(tc.velocityX100, floor, ceiling)
			if got != tc.want {
				t.Errorf("IntervalFromVelocity(%d) = %v; want %v", tc.velocityX100, got, tc.want)
			}
		})
	}
}

func TestBackoffFromErrorCount(t *testing.T) {
	base := 5 * time.Minute
	ceiling := 24 * time.Hour
	cases := []struct {
		name       string
		errorCount int
		want       time.Duration
	}{
		{"first error -> base", 1, 5 * time.Minute},
		{"second error -> 10m", 2, 10 * time.Minute},
		{"third error -> 20m", 3, 20 * time.Minute},
		{"fourth error -> 40m", 4, 40 * time.Minute},
		{"fifth error -> 1h20m", 5, 80 * time.Minute},
		{"twelfth error caps at ceiling", 12, 24 * time.Hour},
		{"zero error_count is treated as 1", 0, 5 * time.Minute},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BackoffFromErrorCount(tc.errorCount, base, ceiling, 0, nil)
			if got != tc.want {
				t.Errorf("BackoffFromErrorCount(%d) = %v; want %v", tc.errorCount, got, tc.want)
			}
		})
	}
}

func TestBackoffFromErrorCount_Jitter(t *testing.T) {
	base := 5 * time.Minute
	ceiling := 24 * time.Hour
	rng := rand.New(rand.NewChaCha8([32]byte{1, 2, 3}))
	delay := BackoffFromErrorCount(2, base, ceiling, 0.25, rng)
	if delay < 10*time.Minute || delay >= 12*time.Minute+30*time.Second {
		t.Errorf("delay %v outside jitter window [10m, 12m30s)", delay)
	}
}

func TestApplyServerFloors(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		candidate   time.Time
		retryAfter  time.Time
		cacheMaxAge time.Duration
		want        time.Time
	}{
		{"no overrides returns candidate", now.Add(2 * time.Hour), time.Time{}, 0, now.Add(2 * time.Hour)},
		{"retry-after later than candidate pushes", now.Add(2 * time.Hour), now.Add(6 * time.Hour), 0, now.Add(6 * time.Hour)},
		{"retry-after earlier than candidate is ignored", now.Add(6 * time.Hour), now.Add(2 * time.Hour), 0, now.Add(6 * time.Hour)},
		{"cache max-age later than candidate pushes", now.Add(2 * time.Hour), time.Time{}, 6 * time.Hour, now.Add(6 * time.Hour)},
		{"both set, later wins", now.Add(2 * time.Hour), now.Add(4 * time.Hour), 6 * time.Hour, now.Add(6 * time.Hour)},
		{"both set, retry-after later wins", now.Add(2 * time.Hour), now.Add(8 * time.Hour), 6 * time.Hour, now.Add(8 * time.Hour)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ApplyServerFloors(tc.candidate, tc.retryAfter, tc.cacheMaxAge, now)
			if !got.Equal(tc.want) {
				t.Errorf("ApplyServerFloors = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		header string
		wantOK bool
		want   time.Time
	}{
		{"empty -> false", "", false, time.Time{}},
		{"whitespace -> false", "   ", false, time.Time{}},
		{"seconds form", "60", true, now.Add(60 * time.Second)},
		{"seconds form with whitespace", "  120  ", true, now.Add(120 * time.Second)},
		{"negative seconds rejected", "-5", false, time.Time{}},
		{"http-date form", "Tue, 09 May 2026 13:00:00 GMT", true, time.Date(2026, 5, 9, 13, 0, 0, 0, time.UTC)},
		{"malformed string -> false", "tomorrow please", false, time.Time{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseRetryAfter(tc.header, now)
			if ok != tc.wantOK {
				t.Errorf("ok = %v; want %v", ok, tc.wantOK)
			}
			if ok && !got.Equal(tc.want) {
				t.Errorf("time = %v; want %v", got, tc.want)
			}
		})
	}
}
