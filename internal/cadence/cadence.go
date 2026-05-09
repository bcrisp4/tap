// Package cadence holds the pure functions that derive a feed's next-poll
// time from its velocity (entries/day), error history, and origin headers.
// Leaf package: imports only stdlib so internal/db and internal/poll can
// both depend on it without cycling.
package cadence

import (
	"math/rand/v2"
	"time"
)

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

// BackoffFromErrorCount returns the delay before the next attempt after
// errorCount consecutive errors, doubling base each time and capping at
// ceiling. When jitterFrac > 0 and rng is non-nil, the result is increased
// by a uniform random fraction in [0, jitterFrac) of the computed delay.
func BackoffFromErrorCount(errorCount int, base, ceiling time.Duration, jitterFrac float64, rng *rand.Rand) time.Duration {
	if errorCount < 1 {
		errorCount = 1
	}
	delay := base
	for i := 1; i < errorCount; i++ {
		if delay >= ceiling {
			delay = ceiling
			break
		}
		delay *= 2
	}
	if delay > ceiling {
		delay = ceiling
	}
	if jitterFrac > 0 && rng != nil {
		max := time.Duration(float64(delay) * jitterFrac)
		if max > 0 {
			delay += time.Duration(rng.Int64N(int64(max)))
		}
	}
	return delay
}

// ApplyServerFloors pushes the candidate next-poll time forward (never
// backward) based on origin response headers.
func ApplyServerFloors(candidate, retryAfter time.Time, cacheMaxAge time.Duration, now time.Time) time.Time {
	if !retryAfter.IsZero() && retryAfter.After(candidate) {
		candidate = retryAfter
	}
	if cacheMaxAge > 0 {
		cacheFloor := now.Add(cacheMaxAge)
		if cacheFloor.After(candidate) {
			candidate = cacheFloor
		}
	}
	return candidate
}
