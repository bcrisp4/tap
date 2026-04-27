package poller

import (
	"context"
	"sync"
	"time"
)

// inflightSet tracks feed IDs currently being polled, so the
// dispatcher's claim query can skip them and feeds don't get
// double-dispatched on overlapping ticks.
type inflightSet struct {
	m sync.Map // key: int64, val: struct{}
}

func (s *inflightSet) Mark(id int64)  { s.m.Store(id, struct{}{}) }
func (s *inflightSet) Clear(id int64) { s.m.Delete(id) }
func (s *inflightSet) Snapshot() []int64 {
	out := []int64{}
	s.m.Range(func(k, _ any) bool { out = append(out, k.(int64)); return true })
	return out
}

// listDueFn matches storage.Store.ListDueFeeds. Indirected so tests
// can inject a fake claim source.
type listDueFn func(ctx context.Context, now int64, limit int, exclude []int64) ([]int64, error)

// DispatcherConfig wires the dispatcher's deps.
type DispatcherConfig struct {
	Worker   *Worker
	Interval time.Duration
	Workers  int
	ListDue  listDueFn
}

// Dispatcher drives the worker pool by polling the DB on each tick.
type Dispatcher struct {
	cfg DispatcherConfig
	in  *inflightSet
	ch  chan int64
}

func newDispatcher(cfg DispatcherConfig) *Dispatcher {
	return &Dispatcher{
		cfg: cfg,
		in:  &inflightSet{},
		// Unbuffered: workers and the dispatcher backpressure each
		// other naturally — the dispatcher blocks on send when every
		// worker is busy.
		ch: make(chan int64),
	}
}

// run blocks until ctx is cancelled. It launches the worker goroutines,
// runs an immediate dispatch (so a freshly-started server doesn't wait
// a full Interval before its first poll), then ticks forever.
func (d *Dispatcher) run(ctx context.Context) {
	var wg sync.WaitGroup
	for range d.cfg.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range d.ch {
				_ = d.cfg.Worker.PollOne(ctx, id)
				d.in.Clear(id)
			}
		}()
	}

	tick := time.NewTicker(d.cfg.Interval)
	defer tick.Stop()

	d.dispatch(ctx)

	for {
		select {
		case <-ctx.Done():
			close(d.ch)
			wg.Wait()
			return
		case <-tick.C:
			d.dispatch(ctx)
		}
	}
}

// dispatch claims due feeds and sends them to the worker pool. We mark
// in-flight before sending so a slow ctx.Done() between mark and send
// still rolls back cleanly.
func (d *Dispatcher) dispatch(ctx context.Context) {
	now := time.Now().Unix()
	ids, err := d.cfg.ListDue(ctx, now, d.cfg.Workers*2, d.in.Snapshot())
	if err != nil {
		return
	}
	for _, id := range ids {
		d.in.Mark(id)
		select {
		case d.ch <- id:
		case <-ctx.Done():
			d.in.Clear(id)
			return
		}
	}
}
