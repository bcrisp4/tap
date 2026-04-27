package poller

import "time"

// PollOutcome is the input to NextPollAt.
type PollOutcome struct {
	Now           int64         // unix seconds
	Failed        bool
	ErrorCount    int           // current count BEFORE this poll
	RetryAfter    time.Duration // server-mandated; 0 if absent
	MaxAge        time.Duration // Cache-Control max-age; 0 if absent
	WeeklyEntries int           // count over last 7 days
}

// AdaptiveResult is what NextPollAt returns.
type AdaptiveResult struct {
	NextPollAt int64
	ErrorCount int
}

const (
	minInterval = 15 * time.Minute
	maxInterval = 24 * time.Hour
	// backoffShiftCap bounds the 2^errCount shift so it can't overflow
	// the time.Duration value space; the result is clamped to
	// maxInterval anyway, but a runaway shift would wrap before the
	// clamp kicks in.
	backoffShiftCap = 12
)

// NextPollAt computes the next poll time per the four ordered rules
// from design.md §5. factor is TAP_POLL_FACTOR (≥0; ≤0 treated as 1).
//
// Rules, in order:
//  1. RetryAfter set → now + RetryAfter, error_count reset.
//  2. Failed → now + min(2^errCount h, 24h), error_count++.
//  3. Cache-Control max-age acts as a floor on the adaptive interval.
//  4. Adaptive: interval = (7d / WeeklyEntries) / factor, clamped
//     [15m, 24h]. WeeklyEntries == 0 → 24h.
func NextPollAt(o PollOutcome, factor float64) AdaptiveResult {
	if factor <= 0 {
		factor = 1.0
	}

	switch {
	case o.RetryAfter > 0:
		return AdaptiveResult{NextPollAt: o.Now + int64(o.RetryAfter.Seconds()), ErrorCount: 0}

	case o.Failed:
		ec := o.ErrorCount + 1
		hours := time.Duration(1<<min(ec, backoffShiftCap)) * time.Hour
		if hours > maxInterval {
			hours = maxInterval
		}
		return AdaptiveResult{NextPollAt: o.Now + int64(hours.Seconds()), ErrorCount: ec}

	default:
		var interval time.Duration
		if o.WeeklyEntries == 0 {
			interval = maxInterval
		} else {
			seconds := (7.0 * 24 * 3600) / float64(o.WeeklyEntries) / factor
			interval = time.Duration(seconds * float64(time.Second))
		}
		if o.MaxAge > interval {
			interval = o.MaxAge
		}
		if interval < minInterval {
			interval = minInterval
		}
		if interval > maxInterval {
			interval = maxInterval
		}
		return AdaptiveResult{NextPollAt: o.Now + int64(interval.Seconds()), ErrorCount: 0}
	}
}
