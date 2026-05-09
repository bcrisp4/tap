package poll

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/feed"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/mmcdole/gofeed"
)

type WorkerOpts struct {
	Cadence   time.Duration        // fixed retry/next-poll interval for M1
	Processor *processor.Processor // applied to every entry's HTML body. Required (panics on nil).
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

	now := time.Now().Unix()
	nextPoll := time.Now().Add(w.opts.Cadence).Unix()

	res, err := feed.Fetch(ctx, w.client, sub.FeedURL, feed.FetchOpts{
		PriorETag:         sub.ETag.String,
		PriorLastModified: sub.LastModified.String,
	})
	if err != nil {
		slog.WarnContext(ctx, "poll error", "feed_id", sub.ID, "feed_url", sub.FeedURL, "err", err)
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, err.Error(), nextPoll)
		return
	}

	if res.Status == http.StatusNotModified {
		slog.DebugContext(ctx, "poll 304", "feed_id", sub.ID)
		_ = db.UpdateAfterNotModified(ctx, w.db, sub.ID, now, nextPoll)
		return
	}

	newEntries := make([]db.NewEntry, 0, len(res.Feed.Items))
	for _, item := range res.Feed.Items {
		pubAt := now
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

	inserted, err := db.UpdateAfterPoll(ctx, w.db, sub.ID, db.PollResult{
		NewETag:         nullStr(res.ETag),
		NewLastModified: nullStr(res.LastModified),
		NextPollAt:      nextPoll,
		NowUnix:         now,
		NewEntries:      newEntries,
	})
	if err != nil {
		slog.ErrorContext(ctx, "commit poll", "feed_id", sub.ID, "err", err)
		_ = db.UpdateAfterError(ctx, w.db, sub.ID, err.Error(), nextPoll)
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
