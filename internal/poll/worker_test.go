package poll

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpx"
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

func insertTestUser(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	id, err := db.InsertUser(context.Background(), d, db.NewUser{
		Username: "polltest", PasswordHash: "x", Role: "admin", CreatedAt: 0,
	})
	if err != nil {
		// If user already exists (reused DB), get its ID
		u, uerr := db.GetUserByUsername(context.Background(), d, "polltest")
		if uerr != nil {
			t.Fatalf("insertTestUser: %v", err)
		}
		return u.ID
	}
	return id
}

func newDBUser(t *testing.T) (*sql.DB, int64) {
	t.Helper()
	d := newDB(t)
	uid := insertTestUser(t, d)
	return d, uid
}

func TestWorker_SuccessfulPoll(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"abc"`)
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, ID: subID, FeedURL: srv.URL})

	got, err := db.GetSubscription(context.Background(), d, subID, uid)
	require.NoError(t, err)
	require.True(t, got.LastPollAt.Valid)
	require.True(t, got.ETag.Valid)
	require.Equal(t, `"abc"`, got.ETag.String)

	entries, _, _, err := db.ListEntries(context.Background(), d, db.ListEntriesParams{UserID: uid, Limit: 100})
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestWorker_ErrorIncrementsCount(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, ID: subID, FeedURL: srv.URL})

	got, _ := db.GetSubscription(context.Background(), d, subID, uid)
	require.Equal(t, 1, got.ErrorCount)
	require.True(t, got.LastError.Valid)
}

func TestWorker_SanitisesContent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(hostileAtom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, ID: subID, FeedURL: srv.URL})

	entries, _, _, err := db.ListEntries(context.Background(), d, db.ListEntriesParams{UserID: uid, Limit: 100})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	// Fetch full entry (list response strips body).
	full, err := db.GetEntry(context.Background(), d, entries[0].ID, uid)
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
	uid := insertTestUser(t, d)

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
		UserID: uid,
		Title: "Bad", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
	})
	require.NoError(t, err)
	sub := db.DueSubscription{UserID: uid, ID: subID, FeedURL: srv.URL, ErrorCount: 0}

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
	uid := insertTestUser(t, d)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	// fixedNow must align with the wall clock because feed.Fetch parses
	// Retry-After against time.Now(), and the server-floor branch only
	// fires when res.RetryAfter > next = fixedNow + delay.
	fixedNow := time.Now().UTC()
	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
		ErrorBase: 5 * time.Minute,
		Now:       func() time.Time { return fixedNow },
	})
	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		UserID: uid,
		Title: "Slow", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
	})
	require.NoError(t, err)
	sub := db.DueSubscription{UserID: uid, ID: subID, FeedURL: srv.URL, ErrorCount: 0}

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
	uid := insertTestUser(t, d)

	fixedNow := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		UserID: uid,
		Title: "Active", FeedURL: "", NextPoll: 0, Created: fixedNow.Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(ctx, `UPDATE subscriptions SET etag='abc' WHERE id=?`, subID)
	require.NoError(t, err)
	for i := 0; i < 14; i++ {
		_, err := d.ExecContext(ctx, `INSERT INTO entries
			(user_id, subscription_id, hash, title, url, content, published_at, fetched_at)
			VALUES (?, ?, ?, '', '', '', ?, ?)`,
			uid, subID, fmt.Sprintf("h%d", i),
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
	sub := db.DueSubscription{UserID: uid, 
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
	uid := insertTestUser(t, d)

	// fixedNow must align with the wall clock because feed.Fetch parses
	// Retry-After against time.Now(); the success branch then floors
	// next_poll against the parsed timestamp.
	fixedNow := time.Now().UTC()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title>
            <item><title>e1</title><link>http://x/1</link></item>
        </channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		UserID: uid,
		Title: "X", FeedURL: srv.URL, NextPoll: 0, Created: fixedNow.Unix(),
	})
	require.NoError(t, err)

	// Pre-seed the velocity window with enough entries that
	// IntervalFromVelocity collapses to Floor (15m). Without that, a single
	// entry yields velocity < 100 and the candidate hits Ceiling = 24h, which
	// would mask the Retry-After test.
	for i := 0; i < 700; i++ {
		_, err := d.ExecContext(ctx, `INSERT INTO entries
			(user_id, subscription_id, hash, title, url, content, published_at, fetched_at)
			VALUES (?, ?, ?, '', '', '', ?, ?)`,
			uid, subID, fmt.Sprintf("seed%d", i),
			fixedNow.Add(-time.Duration(i)*time.Minute).Unix(), fixedNow.Unix())
		require.NoError(t, err)
	}

	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
		Now:       func() time.Time { return fixedNow },
	})
	w.Run(ctx, db.DueSubscription{UserID: uid, ID: subID, FeedURL: srv.URL})

	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll))
	expected := fixedNow.Add(time.Hour - 5*time.Second).Unix()
	require.GreaterOrEqual(t, nextPoll, expected, "Retry-After:3600 must floor next_poll on success")
}

func TestWorker_NotModified_VelocityQueryFailure_AdvancesNextPoll(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))
	uid := insertTestUser(t, d)

	// Origin always 304s; that's the path that calls QueryVelocity.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotModified)
	}))
	t.Cleanup(srv.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		UserID: uid,
		Title: "X", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	// Drop the entries table so QueryVelocity's COUNT errors.
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
	w.Run(ctx, db.DueSubscription{UserID: uid, ID: subID, FeedURL: srv.URL, ErrorCount: 0})

	// next_poll_at must move forward — otherwise the scheduler picks this
	// subscription on every tick (tight loop). Apply exponential backoff
	// just like a fetch failure.
	var nextPoll int64
	var ec int
	var lastError sql.NullString
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT next_poll_at, error_count, last_error FROM subscriptions WHERE id = ?`, subID).
		Scan(&nextPoll, &ec, &lastError))
	earliest := fixedNow.Add(5 * time.Minute).Unix()
	latest := fixedNow.Add(5*time.Minute + 5*time.Minute/4).Unix()
	require.GreaterOrEqual(t, nextPoll, earliest, "QueryVelocity failure must back off, not leave next_poll_at unchanged")
	require.Less(t, nextPoll, latest)
	require.Equal(t, 1, ec, "error_count should bump on velocity-query failure")
	require.True(t, lastError.Valid, "last_error should record the velocity-query failure")
}

func TestNewWorker_DefaultsExtractFunc(t *testing.T) {
	t.Parallel()
	d, _ := newDBUser(t)
	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	// Defaults: ExtractConcurrency = 4, ExtractBodyCap = 5 MiB,
	// Extract = extract.Extract (non-nil).
	require.Equal(t, 4, w.opts.ExtractConcurrency)
	require.Equal(t, int64(5<<20), w.opts.ExtractBodyCap)
	require.NotNil(t, w.opts.Extract)
}

func TestWorker_CommitFailure_UsesExponentialBackoff(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	d, err := db.Open(ctx, ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))
	uid := insertTestUser(t, d)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(sampleAtom))
	}))
	t.Cleanup(srv.Close)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		UserID: uid,
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
	w.Run(ctx, db.DueSubscription{UserID: uid, ID: subID, FeedURL: srv.URL, ErrorCount: 1})

	var nextPoll int64
	require.NoError(t, d.QueryRowContext(ctx,
		`SELECT next_poll_at FROM subscriptions WHERE id = ?`, subID).Scan(&nextPoll))
	earliest := fixedNow.Add(10 * time.Minute).Unix()
	latest := fixedNow.Add(10*time.Minute + 10*time.Minute/4).Unix()
	require.GreaterOrEqual(t, nextPoll, earliest, "commit failure must back off, not retry in 5m flat")
	require.Less(t, nextPoll, latest)
}

func TestWorker_Extract_ReplacesContentOnSuccess(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Linkonly</title>
  <id>urn:linkonly</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>One</title>
    <id>urn:linkonly:1</id>
    <link href="https://link.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;teaser&lt;/p&gt;</content>
  </entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	var calls int
	fakeExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64, creds httpx.FeedCreds) (string, error) {
		calls++
		require.Equal(t, "https://link.example/1", url)
		require.Equal(t, "", sel)
		return "<p>extracted body</p>", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   fakeExtract,
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, 
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	require.Equal(t, 1, calls)
	entries, _, _, err := db.ListEntries(context.Background(), d, db.ListEntriesParams{UserID: uid, Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)
	full, err := db.GetEntry(context.Background(), d, entries[0].ID, uid)
	require.NoError(t, err)
	require.Contains(t, full.Content, "extracted body")
	require.NotContains(t, full.Content, "teaser")
}

func TestWorker_Extract_FalseSkipsExtraction(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Linkonly</title>
  <id>urn:linkonly</id>
  <entry><title>One</title><id>urn:linkonly:1</id>
    <link href="https://link.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;teaser&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	var calls int
	fakeExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64, creds httpx.FeedCreds) (string, error) {
		calls++
		return "should-not-appear", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   fakeExtract,
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, 
		ID: subID, FeedURL: srv.URL, Extract: false,
	})

	require.Equal(t, 0, calls, "Extract must not be called when sub.Extract is false")
	entries, _, _, _ := db.ListEntries(context.Background(), d, db.ListEntriesParams{UserID: uid, Limit: 10})
	full, _ := db.GetEntry(context.Background(), d, entries[0].ID, uid)
	require.Contains(t, full.Content, "teaser")
}

func TestWorker_Extract_OneFailureFallsBackToSummary(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Mix</title><id>urn:mix</id>
  <entry><title>Ok</title><id>urn:mix:1</id>
    <link href="https://mix.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;summary-1&lt;/p&gt;</content></entry>
  <entry><title>Bad</title><id>urn:mix:2</id>
    <link href="https://mix.example/2"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;summary-2&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	fakeExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64, creds httpx.FeedCreds) (string, error) {
		if strings.HasSuffix(url, "/2") {
			return "", errors.New("simulated extract failure")
		}
		return "<p>extracted-1</p>", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   fakeExtract,
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, 
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	// Subscription error_count must NOT bump — extract failure is per-entry.
	got, err := db.GetSubscription(context.Background(), d, subID, uid)
	require.NoError(t, err)
	require.Equal(t, 0, got.ErrorCount, "extract failure must not bump subscription error_count")
	require.False(t, got.LastError.Valid, "extract failure must not write last_error")

	// Both entries inserted; per-entry extract_failed reflects the outcome.
	rows, err := d.QueryContext(context.Background(),
		`SELECT title, content, extract_failed FROM entries WHERE subscription_id = ? ORDER BY title`, subID)
	require.NoError(t, err)
	defer rows.Close()
	type row struct {
		title, content string
		failed         int
	}
	var got2 []row
	for rows.Next() {
		var r row
		require.NoError(t, rows.Scan(&r.title, &r.content, &r.failed))
		got2 = append(got2, r)
	}
	require.Len(t, got2, 2)
	require.Equal(t, "Bad", got2[0].title)
	require.Equal(t, 1, got2[0].failed)
	require.Contains(t, got2[0].content, "summary-2")
	require.Equal(t, "Ok", got2[1].title)
	require.Equal(t, 0, got2[1].failed)
	require.Contains(t, got2[1].content, "extracted-1")
}

func TestWorker_Extract_AllFailuresPollStillSucceeds(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>AllBad</title><id>urn:allbad</id>
  <entry><title>One</title><id>urn:allbad:1</id>
    <link href="https://allbad.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;s1&lt;/p&gt;</content></entry>
  <entry><title>Two</title><id>urn:allbad:2</id>
    <link href="https://allbad.example/2"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;s2&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	failExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64, creds httpx.FeedCreds) (string, error) {
		return "", errors.New("always fail")
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   failExtract,
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, 
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	got, err := db.GetSubscription(context.Background(), d, subID, uid)
	require.NoError(t, err)
	require.Equal(t, 0, got.ErrorCount)

	var failed int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM entries WHERE subscription_id = ? AND extract_failed = 1`,
		subID).Scan(&failed))
	require.Equal(t, 2, failed)
}

func TestWorker_Extract_ConcurrencyLimit(t *testing.T) {
	t.Parallel()
	// Build an atom feed with 5 entries; assert the fake extractor never
	// sees more than 2 in flight when ExtractConcurrency=2.
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>Burst</title><id>urn:burst</id>`)
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&sb, `<entry><title>e%d</title><id>urn:burst:%d</id>
		<link href="https://burst.example/%d"/>
		<updated>2026-05-01T00:00:00Z</updated>
		<content type="html">&lt;p&gt;s&lt;/p&gt;</content></entry>`, i, i, i)
	}
	sb.WriteString(`</feed>`)
	atom := sb.String()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	var (
		mu       sync.Mutex
		inFlight int
		peak     int
	)
	release := make(chan struct{})
	barrier := func(ctx context.Context, c *http.Client, url, sel string, cap int64, creds httpx.FeedCreds) (string, error) {
		mu.Lock()
		inFlight++
		if inFlight > peak {
			peak = inFlight
		}
		mu.Unlock()
		<-release
		mu.Lock()
		inFlight--
		mu.Unlock()
		return "<p>x</p>", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor:          processor.New(sanitise.DefaultPolicy(), nil),
		Extract:            barrier,
		ExtractConcurrency: 2,
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		w.Run(context.Background(), db.DueSubscription{UserID: uid, 
			ID: subID, FeedURL: srv.URL, Extract: true,
		})
	}()

	// Wait until the limiter has settled at peak=2, then release everything.
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return peak == 2
	}, 2*time.Second, 5*time.Millisecond)

	close(release)
	<-done

	require.Equal(t, 2, peak, "ExtractConcurrency=2 must cap at-most 2 in flight")
}

func TestWorker_Extract_NoLinkSkipsSilently(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>Nolink</title><id>urn:nolink</id>
  <entry><title>NoLink</title><id>urn:nolink:1</id>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;summary&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	var calls int
	fakeExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64, creds httpx.FeedCreds) (string, error) {
		calls++
		return "should-not-appear", nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   fakeExtract,
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, 
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	require.Equal(t, 0, calls, "no-Link entries must skip extraction silently")
	var failed int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM entries WHERE subscription_id = ? AND extract_failed = 1`,
		subID).Scan(&failed))
	require.Equal(t, 0, failed, "no-Link entries must NOT have extract_failed=1")
}

func TestWorker_Extract_OutputStillSanitised(t *testing.T) {
	t.Parallel()
	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom"><title>San</title><id>urn:san</id>
  <entry><title>One</title><id>urn:san:1</id>
    <link href="https://san.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;summary&lt;/p&gt;</content></entry>
</feed>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})

	hostileExtract := func(ctx context.Context, c *http.Client, url, sel string, cap int64, creds httpx.FeedCreds) (string, error) {
		return `<p>real article</p><script>alert(1)</script>`, nil
	}

	w := NewWorker(d, http.DefaultClient, WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Extract:   hostileExtract,
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, 
		ID: subID, FeedURL: srv.URL, Extract: true,
	})

	entries, _, _, _ := db.ListEntries(context.Background(), d, db.ListEntriesParams{UserID: uid, Limit: 10})
	full, _ := db.GetEntry(context.Background(), d, entries[0].ID, uid)
	require.Contains(t, full.Content, "real article")
	require.NotContains(t, full.Content, "<script>", "extracted output must run through processor.Process")
	require.NotContains(t, full.Content, "alert", "extracted output must run through processor.Process")
}

func TestWorkerAppliesFeedCredsToFeedFetch(t *testing.T) {
	t.Parallel()
	gotCookie := ""
	gotAuth := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("Cookie")
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>x</title></feed>`))
	}))
	defer srv.Close()

	d, uid := newDBUser(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL, NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})

	w.Run(context.Background(), db.DueSubscription{UserID: uid, 
		ID: subID, FeedURL: srv.URL,
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.Equal(t, "c", gotCookie)
	require.NotEmpty(t, gotAuth)
}

func TestWorkerAppliesFeedCredsToExtract(t *testing.T) {
	t.Parallel()
	feedCalled := false
	gotArticleCookie := ""
	gotArticleAuth := ""

	// Capture srv.URL via closure so the feed body can point at /article on
	// the same origin without templating.
	mux := http.NewServeMux()
	var articleURL string
	mux.HandleFunc("/feed", func(w http.ResponseWriter, r *http.Request) {
		feedCalled = true
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprintf(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom">
<title>x</title><id>urn:x</id>
<entry>
  <title>e</title>
  <id>e1</id>
  <link href="%s"/>
  <updated>2024-01-01T00:00:00Z</updated>
</entry>
</feed>`, articleURL)
	})
	mux.HandleFunc("/article", func(w http.ResponseWriter, r *http.Request) {
		gotArticleCookie = r.Header.Get("Cookie")
		gotArticleAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><article><p>Sufficient text for readability extraction here, lorem ipsum dolor sit amet, consectetur adipiscing elit.</p></article></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	articleURL = srv.URL + "/article"

	d, uid := newDBUser(t)
	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid,
		Title: "x", FeedURL: srv.URL + "/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)

	w := NewWorker(d, srv.Client(), WorkerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	w.Run(context.Background(), db.DueSubscription{UserID: uid, 
		ID: subID, FeedURL: srv.URL + "/feed",
		Extract: true,
		Cookie:  "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})

	require.True(t, feedCalled)
	require.Equal(t, "c", gotArticleCookie, "extract must receive the same creds as the feed fetch")
	require.NotEmpty(t, gotArticleAuth)
}
