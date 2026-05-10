package ratelimit_test

import (
	"sync"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/ratelimit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLimiter() *ratelimit.Limiter {
	return ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      100,             // effectively unlimited for unit tests
		SourceBurst:     100,
		FailThreshold:   3,
		LockoutBase:     50 * time.Millisecond,
		LockoutMax:      200 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	})
}

func TestLimiter_AllowsInitially(t *testing.T) {
	l := newTestLimiter()
	defer l.Stop()
	allowed, retryAfter := l.Allow("1.2.3.4", "alice")
	assert.True(t, allowed)
	assert.Zero(t, retryAfter)
}

func TestLimiter_LockoutAfterThreshold(t *testing.T) {
	l := newTestLimiter()
	defer l.Stop()
	for i := 0; i < 3; i++ {
		l.RecordFailure("1.2.3.4", "alice")
	}
	allowed, retryAfter := l.Allow("1.2.3.4", "alice")
	assert.False(t, allowed)
	assert.Positive(t, retryAfter)
}

func TestLimiter_LockoutEscalates(t *testing.T) {
	l := ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      100,
		SourceBurst:     100,
		FailThreshold:   2,
		LockoutBase:     50 * time.Millisecond,
		LockoutMax:      500 * time.Millisecond,
		CleanupInterval: time.Hour, // don't GC during test
	})
	defer l.Stop()

	// First lockout: 50ms
	l.RecordFailure("x", "bob")
	l.RecordFailure("x", "bob")
	_, r1 := l.Allow("x", "bob")
	assert.InDelta(t, 50*time.Millisecond, r1, float64(10*time.Millisecond))

	// Wait for first lockout to expire
	time.Sleep(60 * time.Millisecond)

	// Second lockout: 100ms
	l.RecordFailure("x", "bob")
	l.RecordFailure("x", "bob")
	_, r2 := l.Allow("x", "bob")
	assert.Greater(t, r2, r1, "second lockout should be longer than first")
}

func TestLimiter_RecordSuccessResets(t *testing.T) {
	l := newTestLimiter()
	defer l.Stop()
	l.RecordFailure("x", "carol")
	l.RecordFailure("x", "carol")
	l.RecordSuccess("carol")

	// Should need full threshold of failures again
	l.RecordFailure("x", "carol")
	l.RecordFailure("x", "carol")
	allowed, _ := l.Allow("x", "carol")
	assert.True(t, allowed, "reset should require full threshold again")
}

func TestLimiter_SourceRateLimit(t *testing.T) {
	l := ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      1,           // 1 per second
		SourceBurst:     2,
		FailThreshold:   100,         // never triggers lockout
		LockoutBase:     time.Hour,
		LockoutMax:      time.Hour,
		CleanupInterval: time.Hour,
	})
	defer l.Stop()

	// Exhaust burst
	ok1, _ := l.Allow("5.5.5.5", "dave")
	ok2, _ := l.Allow("5.5.5.5", "dave")
	assert.True(t, ok1)
	assert.True(t, ok2)

	// Third attempt should be rate-limited
	ok3, retry := l.Allow("5.5.5.5", "dave")
	assert.False(t, ok3)
	assert.Positive(t, retry)
}

func TestLimiter_CleanupPrunesStale(t *testing.T) {
	l := ratelimit.NewLimiter(ratelimit.Opts{
		SourceRate:      100,
		SourceBurst:     100,
		FailThreshold:   2,
		LockoutBase:     20 * time.Millisecond,
		LockoutMax:      20 * time.Millisecond,
		CleanupInterval: 10 * time.Millisecond,
	})
	defer l.Stop()

	l.RecordFailure("old", "old")
	l.RecordFailure("old", "old")
	// Wait for lockout to expire + cleanup to run
	time.Sleep(80 * time.Millisecond)

	// Should be allowed again (state pruned)
	allowed, _ := l.Allow("old", "old")
	assert.True(t, allowed)
}

func TestLimiter_ConcurrentSafe(t *testing.T) {
	l := newTestLimiter()
	defer l.Stop()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(_ int) {
			defer wg.Done()
			l.RecordFailure("src", "user")
			l.Allow("src", "user")
			l.RecordSuccess("user")
		}(i)
	}
	wg.Wait()
}

func TestLimiter_StopClean(_ *testing.T) {
	l := newTestLimiter()
	l.Stop()
	// Calling Stop twice must not panic
	l.Stop()
}

// Compile-time check that require is used for baseline assertions.
var _ = require.NoError
