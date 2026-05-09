package poll

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/stretchr/testify/require"
)

const sampleAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Sample</title>
  <link href="https://sample.example/"/>
  <id>urn:sample</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>One</title>
    <id>urn:sample:1</id>
    <link href="https://sample.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;a&lt;/p&gt;</content>
  </entry>
</feed>`

const hostileAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Hostile</title>
  <id>urn:hostile</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>Bad</title>
    <id>urn:hostile:1</id>
    <link href="https://hostile.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;ok&lt;/p&gt;&lt;script&gt;alert(1)&lt;/script&gt;&lt;a href=&quot;https://e.com/?utm_source=foo&amp;id=1&quot;&gt;link&lt;/a&gt;</content>
  </entry>
</feed>`

func newDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return d
}

func TestWorker_SuccessfulPoll(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	d := newDB(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(context.Background(), db.DueSubscription{ID: subID, FeedURL: srv.URL})

	got, err := db.GetSubscription(context.Background(), d, subID)
	require.NoError(t, err)
	require.True(t, got.LastPollAt.Valid)
	require.True(t, got.ETag.Valid)
	require.Equal(t, `"abc"`, got.ETag.String)

	entries, _, _, err := db.ListEntries(context.Background(), d, db.ListEntriesParams{Limit: 100})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestWorker_ErrorIncrementsCount(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := newDB(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(context.Background(), db.DueSubscription{ID: subID, FeedURL: srv.URL})

	got, _ := db.GetSubscription(context.Background(), d, subID)
	require.Equal(t, 1, got.ErrorCount)
	require.True(t, got.LastError.Valid)
}

func TestWorker_SanitisesContent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hostileAtom))
	}))
	defer srv.Close()

	d := newDB(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(context.Background(), db.DueSubscription{ID: subID, FeedURL: srv.URL})

	entries, _, _, err := db.ListEntries(context.Background(), d, db.ListEntriesParams{Limit: 100})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Fetch full entry (list response strips body).
	full, err := db.GetEntry(context.Background(), d, entries[0].ID)
	require.NoError(t, err)
	body := full.Content
	require.NotContains(t, body, "<script>", "script tag survived sanitise: %s", body)
	require.NotContains(t, body, "alert", "script body survived sanitise: %s", body)
	require.NotContains(t, body, "utm_source", "tracking param survived urlcleaner: %s", body)
	require.Contains(t, body, "<p>ok</p>", "legitimate paragraph stripped: %s", body)
}

func TestWorker_ErrorPath_ExponentialBackoff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	fixedNow := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
		ErrorBase: 5 * time.Minute,
		Now:       func() time.Time { return fixedNow },
	})
	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "Bad", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
	})
	require.NoError(t, err)
	sub := db.DueSubscription{ID: subID, FeedURL: srv.URL, ErrorCount: 0}

	w.Run(ctx, sub)

	var ec int
	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT error_count, next_poll_at FROM subscriptions WHERE id = ?`, subID).
		Scan(&ec, &nextPoll))
	require.Equal(t, 1, ec)
	earliest := fixedNow.Add(5 * time.Minute).Unix()
	latest := fixedNow.Add(5*time.Minute + 5*time.Minute/4).Unix()
	require.GreaterOrEqual(t, nextPoll, earliest)
	require.Less(t, nextPoll, latest)
}

func TestWorker_RetryAfterOverridesBackoff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	// fixedNow must align with the wall clock because feed.Fetch parses
	// Retry-After against time.Now() (Phase 4), and the server-floor branch
	// only fires when res.RetryAfter > next = fixedNow + delay.
	fixedNow := time.Now().UTC()
	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
		ErrorBase: 5 * time.Minute,
		Now:       func() time.Time { return fixedNow },
	})
	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "Slow", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
	})
	require.NoError(t, err)
	sub := db.DueSubscription{ID: subID, FeedURL: srv.URL, ErrorCount: 0}

	w.Run(ctx, sub)

	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll))
	// Allow a small tolerance: feed.Fetch records res.RetryAfter shortly after
	// fixedNow is captured, so res.RetryAfter ≈ fixedNow + 3600s ± a few ms.
	expected := fixedNow.Add(time.Hour - 5*time.Second).Unix()
	require.GreaterOrEqual(t, nextPoll, expected, "Retry-After:3600 must floor next_poll")
}

func TestWorker_NotModified_RecomputesVelocityAndCadence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	fixedNow := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "Active", FeedURL: "", NextPoll: 0, Created: fixedNow.Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(ctx, `UPDATE subscriptions SET etag='abc' WHERE id=?`, subID)
	require.NoError(t, err)
	for i := 0; i < 14; i++ {
		_, err := d.ExecContext(ctx, `INSERT INTO entries
			(subscription_id, hash, title, url, content, published_at, fetched_at)
			VALUES (?, ?, '', '', '', ?, ?)`,
			subID, fmt.Sprintf("h%d", i),
			fixedNow.Add(-time.Duration(i)*12*time.Hour).Unix(), fixedNow.Unix())
		require.NoError(t, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "abc", r.Header.Get("If-None-Match"))
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(srv.Close)
	_, err = d.ExecContext(ctx, `UPDATE subscriptions SET feed_url=? WHERE id=?`, srv.URL, subID)
	require.NoError(t, err)

	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
		Now:       func() time.Time { return fixedNow },
	})
	sub := db.DueSubscription{
		ID: subID, FeedURL: srv.URL,
		ETag: sql.NullString{String: "abc", Valid: true},
	}
	w.Run(ctx, sub)

	var velocity int
	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT velocity_24h_x100, next_poll_at FROM subscriptions WHERE id = ?`,
		subID).Scan(&velocity, &nextPoll))
	require.Equal(t, 200, velocity)
	expected := fixedNow.Add(12 * time.Hour).Unix()
	require.Equal(t, expected, nextPoll)
}

func TestWorker_Success_RetryAfterAdvisoryFloor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	// fixedNow must align with the wall clock because feed.Fetch parses
	// Retry-After against time.Now() (Phase 4); the success branch then
	// floors next_poll against the parsed timestamp.
	fixedNow := time.Now().UTC()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title>
            <item><title>e1</title><link>http://x/1</link></item>
        </channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "X", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
	})
	require.NoError(t, err)

	// Pre-seed the velocity window with enough entries that
	// IntervalFromVelocity collapses to Floor (15m). Without that, a single
	// entry yields velocity < 100 and the candidate hits Ceiling = 24h, which
	// would mask the Retry-After test.
	for i := 0; i < 700; i++ {
		_, err := d.ExecContext(ctx, `INSERT INTO entries
			(subscription_id, hash, title, url, content, published_at, fetched_at)
			VALUES (?, ?, '', '', '', ?, ?)`,
			subID, fmt.Sprintf("seed%d", i),
			fixedNow.Add(-time.Duration(i)*time.Minute).Unix(), fixedNow.Unix())
		require.NoError(t, err)
	}

	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
		Now:       func() time.Time { return fixedNow },
	})
	w.Run(ctx, db.DueSubscription{ID: subID, FeedURL: srv.URL})

	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll))
	expected := fixedNow.Add(time.Hour - 5*time.Second).Unix()
	require.GreaterOrEqual(t, nextPoll, expected, "Retry-After:3600 must floor next_poll on success")
}

func TestWorker_CommitFailure_UsesExponentialBackoff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleAtom))
	}))
	t.Cleanup(srv.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "X", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	// Bump error_count so the backoff lands at 2*ErrorBase (10m), giving a
	// distinguishable [10m, 12m30s) window vs the prior 5m flat behaviour.
	require.NoError(t, db.UpdateAfterError(ctx, d, subID, "prior", 0))

	// Drop the entries table so the INSERT inside UpdateAfterPoll fails. Fetch
	// still succeeds — this is the commit-failure branch, not fetch-failure.
	_, err = d.ExecContext(ctx, `DROP TABLE entries`)
	require.NoError(t, err)

	fixedNow := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)
	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
		ErrorBase: 5 * time.Minute,
		Now:       func() time.Time { return fixedNow },
	})
	w.Run(ctx, db.DueSubscription{ID: subID, FeedURL: srv.URL, ErrorCount: 1})

	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll))
	earliest := fixedNow.Add(10 * time.Minute).Unix()
	latest := fixedNow.Add(10*time.Minute + 10*time.Minute/4).Unix()
	require.GreaterOrEqual(t, nextPoll, earliest, "commit failure must back off, not retry in 5m flat")
	require.Less(t, nextPoll, latest)
}
