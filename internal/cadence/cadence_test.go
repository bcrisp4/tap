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
