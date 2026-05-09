package cadence

import (
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
