// Package poller drives Tap's background feed scheduler: a dispatcher
// claims due feeds from storage, hands them to a worker pool that
// fetches, parses, optionally extracts content, and atomically commits
// the outcome. A daily archival sweep prunes old read entries and ages
// out the media-proxy cache.
package poller

import (
	"sync"
	"time"
)

// RunStateSnapshot is the read-only view returned by Snapshot.
type RunStateSnapshot struct {
	ActivePolls  int
	LastPollAt   int64
	RecentErrors []string
}

// RunState tracks live poller counters. Safe for concurrent use.
type RunState struct {
	mu     sync.Mutex
	active int
	lastAt int64
	errs   []string
	cap    int
}

// NewRunState returns a RunState that keeps the most recent capN
// errors. capN is clamped to a minimum of 1.
func NewRunState(capN int) *RunState {
	if capN < 1 {
		capN = 1
	}
	return &RunState{cap: capN}
}

// PollStarted increments the active-poll counter.
func (r *RunState) PollStarted() {
	r.mu.Lock()
	r.active++
	r.mu.Unlock()
}

// PollFinished decrements the active-poll counter, stamps LastPollAt,
// and (when err != nil) appends the error message to the ring buffer.
func (r *RunState) PollFinished(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active--
	r.lastAt = time.Now().Unix()
	if err != nil {
		r.errs = append(r.errs, err.Error())
		if len(r.errs) > r.cap {
			r.errs = r.errs[len(r.errs)-r.cap:]
		}
	}
}

// Snapshot returns a defensive copy of the current state.
func (r *RunState) Snapshot() RunStateSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := RunStateSnapshot{ActivePolls: r.active, LastPollAt: r.lastAt}
	if len(r.errs) > 0 {
		out.RecentErrors = append([]string(nil), r.errs...)
	}
	return out
}
