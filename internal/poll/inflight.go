// Package poll owns the feed-polling scheduler and worker pool.
package poll

import "sync"

// Inflight tracks subscription IDs currently being polled. Used to prevent
// the dispatcher from re-dispatching a feed whose worker hasn't completed.
//
// Lives in process memory only — a process restart clears it, which is safe
// because polls are idempotent (the entry-hash uniqueness contract drops dupes).
type Inflight struct {
	mu  sync.Mutex
	set map[int64]struct{}
}

func NewInflight() *Inflight {
	return &Inflight{set: make(map[int64]struct{})}
}

// TryAcquire returns true if id was not already in flight (and now is).
func (i *Inflight) TryAcquire(id int64) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if _, ok := i.set[id]; ok {
		return false
	}
	i.set[id] = struct{}{}
	return true
}

func (i *Inflight) Release(id int64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.set, id)
}

// IDs returns a snapshot of currently in-flight IDs (for diagnostics).
func (i *Inflight) IDs() []int64 {
	i.mu.Lock()
	defer i.mu.Unlock()
	out := make([]int64, 0, len(i.set))
	for id := range i.set {
		out = append(out, id)
	}
	return out
}
