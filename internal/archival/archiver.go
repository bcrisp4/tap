package archival

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"
)


// ArchiverOpts configures the archival sweep.
type ArchiverOpts struct {
	Horizon     time.Duration    // entries older than Now()-Horizon are deleted; default 90d
	CacheAgeCap time.Duration    // cache files older than Now()-CacheAgeCap are unlinked; default 14d
	Interval    time.Duration    // sweep cadence; default 24h
	CacheDir    string           // proxy cache root directory
	Now         func() time.Time // clock injection; defaults to time.Now
	OnEvict     func(n int)      // optional; M12 wires tap_proxy_cache_evictions_total{reason="age_sweep"}
	sweepHook   func()           // test seam: called at sweep entry before passes run
}

// Archiver is the fourth concurrent concern in Tap: daily sweep of old entries
// and old cache files.
type Archiver struct {
	db   *sql.DB
	opts ArchiverOpts

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	startOnce sync.Once
	stopOnce  sync.Once
}

// NewArchiver creates an Archiver. Call Start() to begin the sweep ticker.
func NewArchiver(d *sql.DB, opts ArchiverOpts) *Archiver {
	if opts.Interval <= 0 {
		opts.Interval = 24 * time.Hour
	}
	if opts.Horizon <= 0 {
		opts.Horizon = 90 * 24 * time.Hour
	}
	if opts.CacheAgeCap <= 0 {
		opts.CacheAgeCap = 14 * 24 * time.Hour
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Archiver{db: d, opts: opts, ctx: ctx, cancel: cancel}
}

// Start launches the archival ticker goroutine. Idempotent.
func (a *Archiver) Start() {
	a.startOnce.Do(func() {
		a.wg.Add(1)
		go a.loop()
	})
}

// Stop cancels the ticker and waits for any in-progress sweep to complete.
func (a *Archiver) Stop() {
	a.stopOnce.Do(func() {
		a.cancel()
		a.wg.Wait()
	})
}

func (a *Archiver) loop() {
	defer a.wg.Done()
	ticker := time.NewTicker(a.opts.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.sweep()
		}
	}
}

func (a *Archiver) sweep() {
	if a.opts.sweepHook != nil {
		a.opts.sweepHook()
	}

	now := a.opts.Now()
	horizonUnix := now.Add(-a.opts.Horizon).Unix()
	ageCapUnix := now.Add(-a.opts.CacheAgeCap).Unix()
	start := now // use injected clock for consistent duration_ms in tests

	slog.Info("archival.sweep.start")

	// Use a detached context for the sweep passes so that Stop() cancelling
	// a.ctx prevents new sweeps but does not abort a sweep already in progress.
	// The WaitGroup ensures Stop() still blocks until the sweep completes.
	sweepCtx := context.WithoutCancel(a.ctx)

	deleted, tombstoned, err := dbPass(sweepCtx, a.db, horizonUnix, now.Unix())
	if err != nil {
		slog.Warn("archival: db pass error", "err", err)
	}

	evicted, err := fsPass(a.opts.CacheDir, ageCapUnix, a.opts.OnEvict)
	if err != nil {
		slog.Warn("archival: fs pass error", "err", err)
	}

	slog.Info("archival.sweep.complete",
		"entries_deleted", deleted,
		"tombstones_written", tombstoned,
		"cache_files_evicted", evicted,
		"duration_ms", a.opts.Now().Sub(start).Milliseconds(),
	)
}
