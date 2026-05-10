package ring

import (
	"sync"
	"time"
)

// Event is a structured error or warning captured from the slog stream.
type Event struct {
	Time  time.Time
	Level string         // "warn" or "error"
	Event string         // event taxonomy key, e.g. "poll.failure"
	Attrs map[string]any // all slog attributes on the log record
}

// Buffer is a fixed-capacity ring buffer of Events. Safe for concurrent use.
type Buffer struct {
	mu       sync.Mutex
	events   []Event
	capacity int
	head     int // index of the next write slot
	count    int // how many slots are filled
}

// NewBuffer returns a Buffer with the given capacity.
func NewBuffer(capacity int) *Buffer {
	return &Buffer{
		events:   make([]Event, capacity),
		capacity: capacity,
	}
}

// Add inserts e into the buffer, overwriting the oldest entry when full.
func (b *Buffer) Add(e Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events[b.head] = e
	b.head = (b.head + 1) % b.capacity
	if b.count < b.capacity {
		b.count++
	}
}

// Recent returns up to n events newest-first.
func (b *Buffer) Recent(n int) []Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n <= 0 || b.count == 0 {
		return nil
	}
	if n > b.count {
		n = b.count
	}
	out := make([]Event, n)
	// head points to the next write slot, so (head-1) is the newest.
	for i := 0; i < n; i++ {
		idx := (b.head - 1 - i + b.capacity) % b.capacity
		out[i] = b.events[idx]
	}
	return out
}
