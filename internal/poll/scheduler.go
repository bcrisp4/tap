package poll

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/sanitise"
)

type SchedulerOpts struct {
	TickInterval       time.Duration        // default 60s
	Workers            int                  // default 3
	Processor          *processor.Processor // default processor.New(sanitise.DefaultPolicy(), nil)
	Floor              time.Duration        // min interval between polls; default 15m
	Ceiling            time.Duration        // max interval between polls; default 24h
	ErrorBase          time.Duration        // first-error backoff base, doubled per consecutive error; default 5m
	Now                func() time.Time     // default time.Now (overridable in tests)
	Extract            ExtractFunc
	ExtractConcurrency int
	ExtractBodyCap     int64
}

type Scheduler struct {
	db       *sql.DB
	client   *http.Client
	opts     SchedulerOpts
	inflight *Inflight
	worker   *Worker
	jobs     chan db.DueSubscription
	tickDone chan struct{} // closed when the tick goroutine exits
	poke     chan struct{} // buffered; signal "tick now"
	workerWG sync.WaitGroup

	// parentCtx is the lifetime of this scheduler. parentCancel triggers
	// shutdown of both the tick loop and any in-flight worker context.
	parentCtx    context.Context
	parentCancel context.CancelFunc
	startOnce    sync.Once
	stopOnce     sync.Once
	started      atomic.Bool // tracks whether tickLoop was actually launched
}

// NewScheduler creates a scheduler whose lifetime is bounded by base. Cancel
// base or call Stop() to shut down. Worker goroutines start immediately;
// the tick loop starts when Start() is called.
func NewScheduler(base context.Context, d *sql.DB, c *http.Client, o SchedulerOpts) *Scheduler {
	if o.TickInterval <= 0 {
		o.TickInterval = 60 * time.Second
	}
	if o.Workers <= 0 {
		o.Workers = 3
	}
	if o.Processor == nil {
		o.Processor = processor.New(sanitise.DefaultPolicy(), nil)
	}
	if o.Floor <= 0 {
		o.Floor = 15 * time.Minute
	}
	if o.Ceiling <= 0 {
		o.Ceiling = 24 * time.Hour
	}
	if o.ErrorBase <= 0 {
		o.ErrorBase = 5 * time.Minute
	}
	if o.Now == nil {
		o.Now = time.Now
	}
	parentCtx, parentCancel := context.WithCancel(base)
	s := &Scheduler{
		db:       d,
		client:   c,
		opts:     o,
		inflight: NewInflight(),
		worker: NewWorker(d, c, WorkerOpts{
			Processor:          o.Processor,
			Floor:              o.Floor,
			Ceiling:            o.Ceiling,
			ErrorBase:          o.ErrorBase,
			Now:                o.Now,
			Extract:            o.Extract,
			ExtractConcurrency: o.ExtractConcurrency,
			ExtractBodyCap:     o.ExtractBodyCap,
		}),
		jobs:         make(chan db.DueSubscription, o.Workers*2),
		tickDone:     make(chan struct{}),
		poke:         make(chan struct{}, 1),
		parentCtx:    parentCtx,
		parentCancel: parentCancel,
	}
	for i := 0; i < o.Workers; i++ {
		s.workerWG.Add(1)
		go s.workerLoop()
	}
	return s
}

// workerLoop drains the jobs channel. The per-job ctx is derived from
// s.parentCtx so Stop() cancellation propagates into the in-flight HTTP fetch.
func (s *Scheduler) workerLoop() {
	defer s.workerWG.Done()
	for sub := range s.jobs {
		ctx, cancel := context.WithTimeout(s.parentCtx, 60*time.Second)
		s.worker.Run(ctx, sub)
		cancel()
		s.inflight.Release(sub.ID)
	}
}

// Start begins the periodic tick loop in a goroutine. Idempotent and safe
// to call after Stop — a second invocation, or one after parentCtx has been
// cancelled, is a no-op (avoids racing with tickLoop's deferred close of
// tickDone).
func (s *Scheduler) Start() {
	s.startOnce.Do(func() {
		if s.parentCtx.Err() != nil {
			return
		}
		s.started.Store(true)
		go s.tickLoop()
	})
}

func (s *Scheduler) tickLoop() {
	defer close(s.tickDone)

	s.Tick(s.parentCtx) // initial tick at startup

	t := time.NewTicker(s.opts.TickInterval)
	defer t.Stop()
	for {
		select {
		case <-s.parentCtx.Done():
			return
		case <-t.C:
			s.Tick(s.parentCtx)
		case <-s.poke:
			s.Tick(s.parentCtx)
		}
	}
}

// Tick selects due subscriptions and dispatches them. Returns dispatched count.
// Public for tests; production code uses Start() / Poke().
func (s *Scheduler) Tick(ctx context.Context) int {
	now := s.opts.Now().Unix()
	due, err := db.ListDuePolls(ctx, s.db, now, 100)
	if err != nil {
		slog.ErrorContext(ctx, "list due polls", "err", err)
		return 0
	}
	dispatched := 0
	for _, sub := range due {
		if !s.inflight.TryAcquire(sub.ID) {
			continue
		}
		// Guard the send against shutdown — without the parentCtx case here,
		// if Stop() races with Tick(), `s.jobs <- sub` after close(jobs) panics.
		select {
		case <-s.parentCtx.Done():
			s.inflight.Release(sub.ID)
			return dispatched
		case s.jobs <- sub:
			dispatched++
		default:
			s.inflight.Release(sub.ID)
			slog.WarnContext(ctx, "scheduler queue full, deferring", "feed_id", sub.ID)
		}
	}
	return dispatched
}

// Poke triggers an immediate tick. Non-blocking; if a poke is already pending
// it's a no-op (one queued tick is enough). Use after POSTing a new
// subscription so the user doesn't wait up to TickInterval for the first poll.
// ActiveCount returns the number of feeds currently being polled (in-memory approximation).
func (s *Scheduler) ActiveCount() int64 {
	return int64(len(s.inflight.IDs()))
}

func (s *Scheduler) Poke() {
	select {
	case s.poke <- struct{}{}:
	default:
	}
}

// Stop initiates orderly shutdown:
//  1. Cancel parentCtx — signals tickLoop and cancels in-flight worker ctxs.
//  2. Wait for tickLoop to exit — guarantees no more sends to s.jobs.
//  3. Close s.jobs — safe now that no senders remain.
//  4. Wait for the worker pool to drain.
//
// This ordering is the only safe one — closing jobs before tickLoop has stopped
// would race with `Tick`'s send.
func (s *Scheduler) Stop() {
	s.stopOnce.Do(func() {
		s.parentCancel()
		// Only wait on tickDone if Start() was called and the goroutine exists.
		if s.started.Load() {
			<-s.tickDone
		} else {
			// No tick goroutine was started; close tickDone ourselves so any
			// future Wait calls on tickDone don't block.
			close(s.tickDone)
		}
		close(s.jobs)
		s.workerWG.Wait()
	})
}

// Wait blocks until the job queue drains and no workers are in flight, or
// until timeout elapses. Test-only helper — production uses Stop().
func (s *Scheduler) Wait(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for {
		if len(s.jobs) == 0 && len(s.inflight.IDs()) == 0 {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("scheduler.Wait: timed out after %s", timeout)
		}
		<-tick.C
	}
}
