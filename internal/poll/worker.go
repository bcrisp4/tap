package poll

import (
	"context"
	"database/sql"
	"encoding/binary"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bcrisp4/tap/internal/cadence"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/feed"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/mmcdole/gofeed"
)

type WorkerOpts struct {
	Cadence   time.Duration        // legacy; M4 supersedes with Floor/Ceiling. Retained so callers compile.
	Processor *processor.Processor // applied to every entry's HTML body. Required (panics on nil).
	Floor     time.Duration        // min interval between polls; default 15m
	Ceiling   time.Duration        // max interval between polls; default 24h
	ErrorBase time.Duration        // first-error backoff base, doubled per consecutive error; default 5m
	Now       func() time.Time     // default time.Now (overridable in tests)
	Rand      *rand.Rand           // default fresh ChaCha8-seeded Rand (overridable in tests)
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
	if o.Cadence <= 0 {
		o.Cadence = 30 * time.Minute
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
	if o.Rand == nil {
		var seed [32]byte
		binary.LittleEndian.PutUint64(seed[:8], uint64(time.Now().UnixNano()))
		o.Rand = rand.New(rand.NewChaCha8(seed))
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
		delay := cadence.BackoffFromErrorCount(sub.ErrorCount+1, w.opts.ErrorBase, w.opts.Ceiling, 0.25, w.opts.Rand)
		next := now.Add(delay)
		if !res.RetryAfter.IsZero() && res.RetryAfter.After(next) {
			next = res.RetryAfter
		}
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
			slog.ErrorContext(ctx, "query velocity", "feed_id", sub.ID, "err", verr)
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
		slog.ErrorContext(ctx, "commit poll", "feed_id", sub.ID, "err", perr)
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, perr.Error(), now.Add(w.opts.ErrorBase).Unix())
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
