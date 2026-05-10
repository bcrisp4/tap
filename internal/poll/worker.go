package poll

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bcrisp4/tap/internal/cadence"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/extract"
	"github.com/bcrisp4/tap/internal/feed"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/mmcdole/gofeed"
)

// ExtractFunc matches extract.Extract — declared as a type so tests can
// inject deterministic in-memory extractors without spinning up an
// httptest origin. Same shape of test seam as Now func() time.Time.
type ExtractFunc func(ctx context.Context, client *http.Client,
	articleURL, selector string, bodyCap int64) (string, error)

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
				"feed_id", sub.ID, "feed_url", sub.FeedURL,
				"panic", r, "stack", string(debug.Stack()))
		}
	}()

	now := w.opts.Now()

	res, fetchErr := feed.Fetch(ctx, w.client, sub.FeedURL, feed.FetchOpts{
		PriorETag:         sub.ETag.String,
		PriorLastModified: sub.LastModified.String,
	})
	if fetchErr != nil {
		delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25)
		next := cadence.ApplyServerFloors(now.Add(delay), res.RetryAfter, 0, now)
		slog.WarnContext(ctx, "poll error",
			"feed_id", sub.ID, "feed_url", sub.FeedURL,
			"error_count", sub.ErrorCount+1,
			"next_poll_at", next.Unix(),
			"err", fetchErr)
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, fetchErr.Error(), next.Unix())
		return
	}

	if res.Status == http.StatusNotModified {
		velocity, verr := db.QueryVelocity(ctx, w.db, sub.ID, now)
		if verr != nil {
			// Treat velocity-query failure as a poll failure rather than
			// silently returning — otherwise next_poll_at stays put and the
			// scheduler picks this subscription on every tick (tight loop).
			delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25)
			next := now.Add(delay)
			slog.ErrorContext(ctx, "query velocity",
				"feed_id", sub.ID, "error_count", sub.ErrorCount+1,
				"next_poll_at", next.Unix(), "err", verr)
			_ = db.UpdateAfterError(ctx, w.db, sub.ID, verr.Error(), next.Unix())
			return
		}
		interval := cadence.IntervalFromVelocity(velocity, w.opts.Floor, w.opts.Ceiling)
		next := cadence.ApplyServerFloors(now.Add(interval), res.RetryAfter, res.CacheMaxAge, now)
		slog.DebugContext(ctx, "poll 304",
			"feed_id", sub.ID, "velocity_x100", velocity, "next_poll_at", next.Unix())
		_ = db.UpdateAfterNotModified(ctx, w.db, sub.ID, now.Unix(), next.Unix(), velocity)
		return
	}

	newEntries := make([]db.NewEntry, 0, len(res.Feed.Items))
	for _, item := range res.Feed.Items {
		pubAt := now.Unix()
		if item.PublishedParsed != nil {
			pubAt = item.PublishedParsed.Unix()
		}
		content := item.Content
		if content == "" {
			content = item.Description
		}
		content = w.opts.Processor.Process(content)
		newEntries = append(newEntries, db.NewEntry{
			Hash:        feed.EntryHash(sub.ID, item),
			Title:       item.Title,
			Author:      authorName(item),
			URL:         item.Link,
			Content:     content,
			PublishedAt: pubAt,
		})
	}

	inserted, perr := db.UpdateAfterPoll(ctx, w.db, sub.ID, db.PollResult{
		NewETag:         nullStr(res.ETag),
		NewLastModified: nullStr(res.LastModified),
		NowUnix:         now.Unix(),
		NewEntries:      newEntries,
		Floor:           w.opts.Floor,
		Ceiling:         w.opts.Ceiling,
		RetryAfter:      res.RetryAfter,
		CacheMaxAge:     res.CacheMaxAge,
	})
	if perr != nil {
		// Commit failure is just as much a poll failure as a fetch error —
		// use the same exponential backoff so a stuck DB doesn't get
		// hammered every 5 minutes on a high-velocity feed.
		delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25)
		next := now.Add(delay)
		slog.ErrorContext(ctx, "commit poll",
			"feed_id", sub.ID, "error_count", sub.ErrorCount+1,
			"next_poll_at", next.Unix(), "err", perr)
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, perr.Error(), next.Unix())
		return
	}
	slog.InfoContext(ctx, "poll ok", "feed_id", sub.ID, "inserted", inserted, "total_items", len(res.Feed.Items))
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
