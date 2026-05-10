package poll

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/bcrisp4/tap/internal/cadence"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/extract"
	"github.com/bcrisp4/tap/internal/feed"
	"github.com/bcrisp4/tap/internal/httpx"
	"github.com/bcrisp4/tap/internal/metrics"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/mmcdole/gofeed"
	"golang.org/x/sync/errgroup"
)

// ExtractFunc matches extract.Extract — declared as a type so tests can
// inject deterministic in-memory extractors without spinning up an
// httptest origin. Same shape of test seam as Now func() time.Time.
type ExtractFunc func(ctx context.Context, client *http.Client,
	articleURL, selector string, bodyCap int64,
	creds httpx.FeedCreds) (string, error)

type WorkerOpts struct {
	Processor          *processor.Processor // applied to every entry's HTML body. Required (panics on nil).
	Floor              time.Duration        // min interval between polls; default 15m
	Ceiling            time.Duration        // max interval between polls; default 24h
	ErrorBase          time.Duration        // first-error backoff base, doubled per consecutive error; default 5m
	Now                func() time.Time     // default time.Now (overridable in tests)
	Extract            ExtractFunc          // default extract.Extract; test seam
	ExtractConcurrency int                  // default 4
	ExtractBodyCap     int64                // default 5 MiB
}

type Worker struct {
	db     *sql.DB
	client *http.Client
	opts   WorkerOpts
}

// NewWorker requires a non-nil Processor. Defaulting it here would silently
// hide tests that forget to pass one — the public construction surface
// (Scheduler) supplies the production default.
func NewWorker(d *sql.DB, c *http.Client, o WorkerOpts) *Worker {
	if o.Processor == nil {
		panic("poll.NewWorker: Processor is required")
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
	if o.Extract == nil {
		o.Extract = extract.Extract
	}
	if o.ExtractConcurrency <= 0 {
		o.ExtractConcurrency = 4
	}
	if o.ExtractBodyCap <= 0 {
		o.ExtractBodyCap = 5 << 20
	}
	if c == nil {
		c = http.DefaultClient
	}
	return &Worker{db: d, client: c, opts: o}
}

// Run polls a single subscription end-to-end. Always returns; never panics.
func (w *Worker) Run(ctx context.Context, sub db.DueSubscription) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "worker panic recovered",
				"event", "worker.panic",
				"feed_id", sub.ID, "feed_url", sub.FeedURL,
				"panic", r, "stack", string(debug.Stack()))
		}
	}()

	tracer := otel.Tracer("tap/poll")
	ctx, span := tracer.Start(ctx, "poll.feed")
	defer span.End()
	span.SetAttributes(attribute.Int64("feed_id", sub.ID))

	startTime := time.Now()
	slog.DebugContext(ctx, "polling feed",
		"event", "poll.start", "feed_id", sub.ID, "feed_url", sub.FeedURL)

	now := w.opts.Now()

	creds := httpx.FeedCreds{
		Cookie:        sub.Cookie,
		BasicAuthUser: sub.BasicAuthUser,
		BasicAuthPass: sub.BasicAuthPass,
	}

	res, fetchErr := feed.Fetch(ctx, w.client, sub.FeedURL, feed.FetchOpts{
		PriorETag:         sub.ETag.String,
		PriorLastModified: sub.LastModified.String,
		Creds:             creds,
	})
	if fetchErr != nil {
		elapsed := time.Since(startTime).Seconds()
		delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25)
		next := cadence.ApplyServerFloors(now.Add(delay), res.RetryAfter, 0, now)
		metrics.PollsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "failure")))
		metrics.PollDuration.Record(ctx, elapsed, metric.WithAttributes(attribute.String("result", "failure")))
		slog.WarnContext(ctx, "poll error",
			"event", "poll.failure",
			"feed_id", sub.ID, "feed_url", sub.FeedURL,
			"error", fetchErr.Error(),
			"error_count", sub.ErrorCount+1,
			"duration_ms", int64(elapsed*1000),
			"next_poll_at", next.Unix())
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, fetchErr.Error(), next.Unix())
		return
	}

	if res.Status == http.StatusNotModified {
		elapsed := time.Since(startTime).Seconds()
		velocity, verr := db.QueryVelocity(ctx, w.db, sub.ID, now)
		if verr != nil {
			metrics.PollsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "failure")))
			metrics.PollDuration.Record(ctx, elapsed, metric.WithAttributes(attribute.String("result", "failure")))
			delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25)
			next := now.Add(delay)
			slog.ErrorContext(ctx, "query velocity",
				"feed_id", sub.ID, "error_count", sub.ErrorCount+1,
				"next_poll_at", next.Unix(), "err", verr)
			_ = db.UpdateAfterError(ctx, w.db, sub.ID, verr.Error(), next.Unix())
			return
		}
		metrics.PollsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "success")))
		metrics.PollDuration.Record(ctx, elapsed, metric.WithAttributes(attribute.String("result", "success")))
		metrics.ConditionalGetHits.Add(ctx, 1)
		interval := cadence.IntervalFromVelocity(velocity, w.opts.Floor, w.opts.Ceiling)
		next := cadence.ApplyServerFloors(now.Add(interval), res.RetryAfter, res.CacheMaxAge, now)
		slog.InfoContext(ctx, "poll succeeded (304)",
			"event", "poll.success",
			"feed_id", sub.ID, "entries_inserted", 0,
			"duration_ms", int64(elapsed*1000),
			"conditional_hit", true)
		_ = db.UpdateAfterNotModified(ctx, w.db, sub.ID, now.Unix(), next.Unix(), velocity)
		return
	}

	type pending struct {
		item          *gofeed.Item
		content       string
		extractFailed bool
	}
	pendings := make([]pending, len(res.Feed.Items))
	for i, item := range res.Feed.Items {
		raw := item.Content
		if raw == "" {
			raw = item.Description
		}
		pendings[i] = pending{item: item, content: raw}
	}

	if sub.Extract && len(pendings) > 0 {
		g := new(errgroup.Group)
		g.SetLimit(w.opts.ExtractConcurrency)
		for i := range pendings {
			if pendings[i].item.Link == "" {
				continue
			}
			g.Go(func() error {
				extracted, eerr := w.opts.Extract(ctx, w.client,
					pendings[i].item.Link, sub.ExtractSelector, w.opts.ExtractBodyCap,
					creds)
				if eerr != nil {
					slog.WarnContext(ctx, "extract failed",
						"feed_id", sub.ID,
						"entry_url", pendings[i].item.Link,
						"err", eerr)
					pendings[i].extractFailed = true
					return nil
				}
				pendings[i].content = extracted
				return nil
			})
		}
		_ = g.Wait()
	}

	candidates := make([]db.NewEntry, 0, len(pendings))
	for _, p := range pendings {
		pubAt := now.Unix()
		if p.item.PublishedParsed != nil {
			pubAt = p.item.PublishedParsed.Unix()
		}
		candidates = append(candidates, db.NewEntry{
			Hash:          feed.EntryHash(sub.ID, p.item),
			Title:         p.item.Title,
			Author:        authorName(p.item),
			URL:           p.item.Link,
			Content:       w.opts.Processor.Process(p.content),
			PublishedAt:   pubAt,
			ExtractFailed: p.extractFailed,
		})
	}

	// Read-only tombstone lookups run outside the commit transaction (concept §6.15).
	newEntries := make([]db.NewEntry, 0, len(candidates))
	for _, e := range candidates {
		tombstoned, terr := db.IsTombstoned(ctx, w.db, sub.ID, e.Hash)
		if terr != nil {
			slog.WarnContext(ctx, "tombstone check failed, including entry",
				"feed_id", sub.ID, "hash", e.Hash, "err", terr)
			newEntries = append(newEntries, e)
			continue
		}
		if tombstoned {
			continue
		}
		newEntries = append(newEntries, e)
	}

	inserted, perr := db.UpdateAfterPoll(ctx, w.db, sub.ID, db.PollResult{
		UserID:          sub.UserID,
		NewETag:         nullStr(res.ETag),
		NewLastModified: nullStr(res.LastModified),
		NowUnix:         now.Unix(),
		NewEntries:      newEntries,
		Floor:           w.opts.Floor,
		Ceiling:         w.opts.Ceiling,
		RetryAfter:      res.RetryAfter,
		CacheMaxAge:     res.CacheMaxAge,
	})
	elapsed := time.Since(startTime).Seconds()
	if perr != nil {
		metrics.PollsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "failure")))
		metrics.PollDuration.Record(ctx, elapsed, metric.WithAttributes(attribute.String("result", "failure")))
		// Commit failure is just as much a poll failure as a fetch error —
		// use the same exponential backoff so a stuck DB doesn't get
		// hammered every 5 minutes on a high-velocity feed.
		delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25)
		next := now.Add(delay)
		slog.ErrorContext(ctx, "commit poll",
			"event", "poll.failure",
			"feed_id", sub.ID, "error", perr.Error(),
			"error_count", sub.ErrorCount+1,
			"duration_ms", int64(elapsed*1000),
			"next_poll_at", next.Unix())
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, perr.Error(), next.Unix())
		return
	}
	metrics.PollsTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", "success")))
	metrics.PollDuration.Record(ctx, elapsed, metric.WithAttributes(attribute.String("result", "success")))
	metrics.EntriesInserted.Add(ctx, int64(inserted))
	slog.InfoContext(ctx, "poll succeeded",
		"event", "poll.success",
		"feed_id", sub.ID, "entries_inserted", inserted,
		"duration_ms", int64(elapsed*1000),
		"conditional_hit", false)
}

func authorName(i *gofeed.Item) string {
	if i.Author != nil {
		return i.Author.Name
	}
	return ""
}

func nullStr(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
