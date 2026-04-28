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

// PollerError is a single recorded poll failure, exposed via
// /api/v1/system/status so the UI can name the failing feed and link
// to its page. FeedID/FeedTitle are zero/empty for process-wide errors
// (archival sweep, dispatcher panics) that aren't tied to one feed.
type PollerError struct {
	FeedID    int64  `json:"feed_id"`
	FeedTitle string `json:"feed_title"`
	Error     string `json:"error"`
	At        int64  `json:"at"` // unix seconds
}

// RunStateSnapshot is the read-only view returned by Snapshot.
type RunStateSnapshot struct {
	ActivePolls  int
	LastPollAt   int64
	RecentErrors []PollerError
}

// RunState tracks live poller counters. Safe for concurrent use.
type RunState struct {
	mu     sync.Mutex
	active int
	lastAt int64
	errs   []PollerError
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
// and (when err != nil) appends the error to the ring buffer with
// feed metadata. Pass feedID=0 / feedTitle="" when the error isn't
// tied to a specific feed.
func (r *RunState) PollFinished(feedID int64, feedTitle string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.active--
	r.lastAt = time.Now().Unix()
	r.appendErrLocked(feedID, feedTitle, err)
}

// RecordError appends an out-of-band error (archival sweep, dispatcher
// panic, etc.) to the ring buffer without touching the active-poll
// counter or LastPollAt. Nil is a no-op. Pass feedID=0 / feedTitle=""
// when the error isn't tied to a specific feed.
func (r *RunState) RecordError(feedID int64, feedTitle string, err error) {
	if err == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.appendErrLocked(feedID, feedTitle, err)
}

func (r *RunState) appendErrLocked(feedID int64, feedTitle string, err error) {
	if err == nil {
		return
	}
	r.errs = append(r.errs, PollerError{
		FeedID:    feedID,
		FeedTitle: feedTitle,
		Error:     err.Error(),
		At:        time.Now().Unix(),
	})
	if len(r.errs) > r.cap {
		r.errs = r.errs[len(r.errs)-r.cap:]
	}
}

// Snapshot returns a defensive copy of the current state.
func (r *RunState) Snapshot() RunStateSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := RunStateSnapshot{ActivePolls: r.active, LastPollAt: r.lastAt}
	if len(r.errs) > 0 {
		out.RecentErrors = append([]PollerError(nil), r.errs...)
	}
	return out
}
