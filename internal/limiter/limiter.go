// Package limiter holds Tap's per-host concurrency limiter, shared
// between the poller (feed + article fetches) and the media proxy
// (origin fetches) per design.md §6.
package limiter

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

// HostLimiter enforces a per-host concurrency cap, lazy-initialising
// the underlying semaphore on first Acquire for a host.
type HostLimiter struct {
	weight int64
	sems   sync.Map // key: host (string), val: *semaphore.Weighted
}

// NewHostLimiter constructs a limiter with the given per-host weight
// (typically 1). Weights below 1 are clamped to 1.
func NewHostLimiter(weight int64) *HostLimiter {
	if weight < 1 {
		weight = 1
	}
	return &HostLimiter{weight: weight}
}

func (l *HostLimiter) sem(host string) *semaphore.Weighted {
	if v, ok := l.sems.Load(host); ok {
		return v.(*semaphore.Weighted)
	}
	fresh := semaphore.NewWeighted(l.weight)
	actual, _ := l.sems.LoadOrStore(host, fresh)
	return actual.(*semaphore.Weighted)
}

// Acquire blocks until a token for host is available or ctx expires.
func (l *HostLimiter) Acquire(ctx context.Context, host string) error {
	return l.sem(host).Acquire(ctx, 1)
}

// Release returns the caller's token for host. Must pair with a
// prior successful Acquire.
func (l *HostLimiter) Release(host string) {
	l.sem(host).Release(1)
}
