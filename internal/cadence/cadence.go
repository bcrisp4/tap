// Package cadence holds the pure functions that derive a feed's next-poll
// time from its velocity (entries/day), error history, and origin headers.
// Leaf package: imports only stdlib so internal/db and internal/poll can
// both depend on it without cycling.
package cadence

import "time"

// IntervalFromVelocity converts a fixed-point velocity (entries/day × 100) to
// a polling interval, clamped to [floor, ceiling]. Velocities below 1 entry/day
// hit the ceiling directly; the formula 86400/velocity_per_day produces the
// raw interval before clamping.
func IntervalFromVelocity(velocityX100 int, floor, ceiling time.Duration) time.Duration {
	if velocityX100 < 100 {
		return ceiling
	}
	interval := time.Duration(int64(86400)*100/int64(velocityX100)) * time.Second
	if interval < floor {
		return floor
	}
	if interval > ceiling {
		return ceiling
	}
	return interval
}
