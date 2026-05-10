// Package ratelimit provides per-source token-bucket rate limiting and
// per-username escalating lockout for the login endpoint.
// All state is in-memory and resets on process restart.
package ratelimit

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Opts configures the Limiter.
type Opts struct {
	SourceRate      rate.Limit    // requests/second; use rate.Every(time.Minute/N) for N/min
	SourceBurst     int
	FailThreshold   int           // consecutive failures before first lockout
	LockoutBase     time.Duration // first lockout duration
	LockoutMax      time.Duration // cap on escalating lockout
	CleanupInterval time.Duration // how often stale entries are pruned
}

type sourceState struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type usernameState struct {
	failures     int
	lockoutLevel int
	lockedUntil  time.Time
	lastSeen     time.Time
}

// Limiter implements per-source rate limiting and per-username escalating lockout.
type Limiter struct {
	opts      Opts
	mu        sync.Mutex
	sources   map[string]*sourceState
	usernames map[string]*usernameState
	stopCh    chan struct{}
	once      sync.Once
}

// NewLimiter returns a started Limiter. Call Stop() when done.
func NewLimiter(opts Opts) *Limiter {
	l := &Limiter{
		opts:      opts,
		sources:   make(map[string]*sourceState),
		usernames: make(map[string]*usernameState),
		stopCh:    make(chan struct{}),
	}
	go l.cleanup()
	return l
}

func (l *Limiter) getSource(source string) *sourceState {
	s, ok := l.sources[source]
	if !ok {
		s = &sourceState{limiter: rate.NewLimiter(l.opts.SourceRate, l.opts.SourceBurst)}
		l.sources[source] = s
	}
	s.lastSeen = time.Now()
	return s
}

func (l *Limiter) getUsername(username string) *usernameState {
	u, ok := l.usernames[username]
	if !ok {
		u = &usernameState{}
		l.usernames[username] = u
	}
	u.lastSeen = time.Now()
	return u
}

// Allow returns (true, 0) if the request may proceed.
// Returns (false, retryAfter) if the source is rate-limited or the username is locked out.
func (l *Limiter) Allow(source, username string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	src := l.getSource(source)
	if !src.limiter.Allow() {
		// Token bucket exhausted — calculate approximate retry delay.
		r := src.limiter.Reserve()
		delay := r.Delay()
		r.Cancel()
		return false, delay
	}

	usr := l.getUsername(username)
	if !usr.lockedUntil.IsZero() && time.Now().Before(usr.lockedUntil) {
		return false, time.Until(usr.lockedUntil)
	}

	return true, 0
}

// RecordSuccess resets the failure counter and lockout escalation for username.
func (l *Limiter) RecordSuccess(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if u, ok := l.usernames[username]; ok {
		u.failures = 0
		u.lockoutLevel = 0
		u.lockedUntil = time.Time{}
		u.lastSeen = time.Now()
	}
}

// RecordFailure increments the failure counter for source and username.
// If the per-username counter reaches FailThreshold, a lockout is applied.
func (l *Limiter) RecordFailure(source, username string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	_ = l.getSource(source) // touch lastSeen

	usr := l.getUsername(username)
	usr.failures++
	if usr.failures >= l.opts.FailThreshold {
		dur := l.opts.LockoutBase
		for i := 0; i < usr.lockoutLevel; i++ {
			dur *= 2
			if dur > l.opts.LockoutMax {
				dur = l.opts.LockoutMax
				break
			}
		}
		usr.lockedUntil = time.Now().Add(dur)
		usr.failures = 0
		usr.lockoutLevel++
	}
}

// Stop halts the cleanup goroutine. Safe to call multiple times.
func (l *Limiter) Stop() {
	l.once.Do(func() { close(l.stopCh) })
}

func (l *Limiter) cleanup() {
	ticker := time.NewTicker(l.opts.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			l.prune()
		case <-l.stopCh:
			return
		}
	}
}

func (l *Limiter) prune() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cutoff := time.Now().Add(-2 * l.opts.LockoutMax)
	for k, s := range l.sources {
		if s.lastSeen.Before(cutoff) {
			delete(l.sources, k)
		}
	}
	for k, u := range l.usernames {
		if u.lastSeen.Before(cutoff) && (u.lockedUntil.IsZero() || time.Now().After(u.lockedUntil)) {
			delete(l.usernames, k)
		}
	}
}
