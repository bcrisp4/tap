// Package cadence holds the pure functions that derive a feed's next-poll
// time from its velocity (entries/day), error history, and origin headers.
// Leaf package: imports only stdlib so internal/db and internal/poll can
// both depend on it without cycling.
package cadence

import (
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
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
// ceiling. When jitterFrac > 0, the result is increased by a uniform
// random fraction in [0, jitterFrac) of the computed delay using the
// package-level rand.Int64N (which math/rand/v2 documents as safe for
// concurrent use, unlike *Rand methods). The jitter can push the result
// above ceiling; that's intentional, to spread thundering-herd recovery
// across feeds that errored together.
func BackoffFromErrorCount(errorCount int, base, ceiling time.Duration, jitterFrac float64) time.Duration {
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
	if jitterFrac > 0 {
		jitterCap := time.Duration(float64(delay) * jitterFrac)
		if jitterCap > 0 {
			delay += time.Duration(rand.Int64N(int64(jitterCap)))
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

// ParseRetryAfter parses the HTTP Retry-After header per RFC 7231 §7.1.3:
// either a non-negative integer of seconds (delta-seconds) or an HTTP-date.
// Returns (target time, true) on success; (zero, false) on absent or malformed.
func ParseRetryAfter(header string, now time.Time) (time.Time, bool) {
	header = strings.TrimSpace(header)
	if header == "" {
		return time.Time{}, false
	}
	if secs, err := strconv.Atoi(header); err == nil {
		if secs < 0 {
			return time.Time{}, false
		}
		return now.Add(time.Duration(secs) * time.Second), true
	}
	if t, err := http.ParseTime(header); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// ParseCacheMaxAge extracts max-age from a Cache-Control header.
// Returns (duration, true) when present and non-negative; (0, false) otherwise.
// max-age=0 returns (0, true) — the caller decides whether zero floors anything.
// Directive names are matched case-insensitively per RFC 9111 §5.2.
func ParseCacheMaxAge(header string) (time.Duration, bool) {
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if eq := strings.IndexByte(part, '='); eq >= 0 && strings.EqualFold(part[:eq], "max-age") {
			secs, err := strconv.Atoi(part[eq+1:])
			if err != nil || secs < 0 {
				return 0, false
			}
			return time.Duration(secs) * time.Second, true
		}
	}
	return 0, false
}
