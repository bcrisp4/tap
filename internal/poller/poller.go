package poller

import (
	"context"
	"sync"
	"time"

	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/limiter"
	"github.com/bcrisp4/tap/internal/reader"
	"github.com/bcrisp4/tap/internal/storage"
)

// Config wires the poller's deps. Zero-valued duration / count fields
// fall back to the design.md §5 defaults; HostLimiter is optional —
// when nil, the poller builds its own using HostWeight (clamped to 1).
type Config struct {
	Store            *storage.Store
	Client           *httpclient.Client
	Pipeline         *reader.Pipeline     // optional; nil disables content extraction
	HostLimiter      *limiter.HostLimiter // optional; cmd/tap passes one shared with the media proxy
	Interval         time.Duration        // dispatcher tick; default 60s
	Workers          int                  // worker pool size; default 4
	PollFactor       float64              // adaptive multiplier; default 1.0
	ArchiveDays      int                  // entry archival age; default 60
	ProxyCacheDir    string               // optional; "" disables the cache age sweep
	ProxyCacheMaxAge time.Duration        // optional; 0 disables the cache age sweep
	HostWeight       int64                // per-host concurrency cap; default 1; ignored when HostLimiter is set
}

// Poller is the public face: dispatcher + worker pool + archival sweep.
type Poller struct {
	cfg     Config
	state   *RunState
	hostLim *limiter.HostLimiter
	worker  *Worker
}

// New builds a Poller. Cheap; one per process.
func New(cfg Config) *Poller {
	if cfg.Interval == 0 {
		cfg.Interval = 60 * time.Second
	}
	if cfg.Workers == 0 {
		cfg.Workers = 4
	}
	if cfg.PollFactor == 0 {
		cfg.PollFactor = 1.0
	}
	if cfg.ArchiveDays == 0 {
		cfg.ArchiveDays = 60
	}
	if cfg.HostWeight == 0 {
		cfg.HostWeight = 1
	}
	state := NewRunState(64)
	hostLim := cfg.HostLimiter
	if hostLim == nil {
		hostLim = limiter.NewHostLimiter(cfg.HostWeight)
	}
	worker := NewWorker(WorkerConfig{
		Store: cfg.Store, Client: cfg.Client, Limiter: hostLim,
		Pipeline: cfg.Pipeline, RunState: state, PollFactor: cfg.PollFactor,
	})
	return &Poller{cfg: cfg, state: state, hostLim: hostLim, worker: worker}
}

// HostLimiter returns the shared per-host limiter so cmd/tap can pass
// the same instance into the media proxy (design §6).
func (p *Poller) HostLimiter() *limiter.HostLimiter { return p.hostLim }

// State exposes the live RunState (for /api/v1/system/status in Plan 08).
func (p *Poller) State() *RunState { return p.state }

// Start blocks until ctx is cancelled, running the dispatcher and the
// archival sweep concurrently.
func (p *Poller) Start(ctx context.Context) {
	disp := newDispatcher(DispatcherConfig{
		Worker:   p.worker,
		Interval: p.cfg.Interval,
		Workers:  p.cfg.Workers,
		ListDue:  p.cfg.Store.ListDueFeeds,
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); disp.run(ctx) }()
	go func() { defer wg.Done(); p.runArchival(ctx) }()
	wg.Wait()
}

func (p *Poller) runArchival(ctx context.Context) {
	tick := time.NewTicker(24 * time.Hour)
	defer tick.Stop()

	doSweep := func() {
		var ps *ProxyCacheSweep
		if p.cfg.ProxyCacheDir != "" && p.cfg.ProxyCacheMaxAge > 0 {
			ps = &ProxyCacheSweep{Dir: p.cfg.ProxyCacheDir, MaxAge: p.cfg.ProxyCacheMaxAge}
		}
		_ = ArchiveOnce(ctx, p.cfg.Store,
			time.Duration(p.cfg.ArchiveDays)*24*time.Hour, ps)
	}

	doSweep() // run once at boot

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			doSweep()
		}
	}
}
