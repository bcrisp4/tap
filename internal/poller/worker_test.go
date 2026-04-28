package poller_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/limiter"
	"github.com/bcrisp4/tap/internal/poller"
	"github.com/bcrisp4/tap/internal/reader"
	"github.com/bcrisp4/tap/internal/storage"
)

// minimalAtom feeds back two entries; vary the title to detect re-runs.
func minimalAtom(serial int) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Test feed</title>
  <id>https://t/</id>
  <entry><id>e1</id><title>Entry one (#%d)</title><link href="https://t/1"/><published>2026-04-26T08:00:00Z</published><content type="html">&lt;p&gt;body&lt;/p&gt;</content></entry>
  <entry><id>e2</id><title>Entry two (#%d)</title><link href="https://t/2"/><published>2026-04-26T07:00:00Z</published><content type="html">&lt;p&gt;body two&lt;/p&gt;</content></entry>
</feed>`, serial, serial)
}

func setupWorkerTest(t *testing.T) (*storage.Store, *poller.Worker, *int) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	store := storage.New(d)

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("If-None-Match") == `"v1"` {
			w.Header().Set("ETag", `"v1"`)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write([]byte(minimalAtom(hits)))
	}))

	client := httpclient.NewClient(httpclient.Config{
		Timeout: 2 * time.Second, MaxBodyBytes: 1 << 20, AllowPrivate: true,
	})
	w := poller.NewWorker(poller.WorkerConfig{
		Store:      store,
		Client:     client,
		Limiter:    limiter.NewHostLimiter(1),
		PollFactor: 1.0,
	})

	t.Cleanup(func() { srv.Close(); d.Close() })

	// Subscribe a feed pointing at the httptest server.
	_, err = store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "Test", FeedURL: srv.URL,
		PollInterval: 3600,
	})
	require.NoError(t, err)
	return store, w, &hits
}

func TestWorker_FirstPollInsertsEntries(t *testing.T) {
	store, w, hits := setupWorkerTest(t)

	feeds, _ := store.ListFeeds(context.Background(), 1)
	require.Len(t, feeds, 1)

	require.NoError(t, w.PollOne(context.Background(), feeds[0].ID))
	require.Equal(t, 1, *hits)

	entries, _ := store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
	require.Len(t, entries, 2)
}

func TestWorker_SecondPollRespectsConditional(t *testing.T) {
	store, w, hits := setupWorkerTest(t)
	feeds, _ := store.ListFeeds(context.Background(), 1)
	require.NoError(t, w.PollOne(context.Background(), feeds[0].ID))

	// Re-read feed; ETag must be stored.
	feeds, _ = store.ListFeeds(context.Background(), 1)
	require.NotNil(t, feeds[0].ETag)
	require.Equal(t, `"v1"`, *feeds[0].ETag)

	// A 304 from origin: poller should still advance next_poll_at and
	// not insert duplicates.
	require.NoError(t, w.PollOne(context.Background(), feeds[0].ID))
	require.Equal(t, 2, *hits)

	entries, _ := store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
	require.Len(t, entries, 2, "duplicates blocked by UNIQUE(feed_id, hash)")
}

func TestWorker_FailureIncrementsErrorCount(t *testing.T) {
	store, w, _ := setupWorkerTest(t)
	feeds, _ := store.ListFeeds(context.Background(), 1)

	// Replace the feed_url with something that won't resolve.
	f, _ := store.GetFeed(context.Background(), 1, feeds[0].ID)
	f.FeedURL = "http://127.0.0.1:1/" // local + invalid port
	require.NoError(t, store.UpdateFeed(context.Background(), f))

	err := w.PollOne(context.Background(), f.ID)
	require.NoError(t, err, "PollOne records failures internally; never returns the underlying error")

	f, _ = store.GetFeed(context.Background(), 1, f.ID)
	require.Equal(t, 1, f.ErrorCount)
	require.NotNil(t, f.LastError)
	require.NotNil(t, f.NextPollAt)
}

func TestWorker_RetryAfterHonouredOnNon2xx(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	t.Cleanup(func() { d.Close() })
	store := storage.New(d)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "120")
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	worker := poller.NewWorker(poller.WorkerConfig{
		Store:      store,
		Client:     httpclient.NewClient(httpclient.Config{Timeout: 2 * time.Second, MaxBodyBytes: 1 << 20, AllowPrivate: true}),
		Limiter:    limiter.NewHostLimiter(1),
		PollFactor: 1.0,
	})

	feedID, err := store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "rl", FeedURL: srv.URL, PollInterval: 3600,
	})
	require.NoError(t, err)

	before := time.Now().Unix()
	require.NoError(t, worker.PollOne(context.Background(), feedID))

	f, _ := store.GetFeed(context.Background(), 1, feedID)
	require.Equal(t, 1, f.ErrorCount, "non-2xx must increment error_count even with Retry-After")
	require.NotNil(t, f.NextPollAt)
	delta := *f.NextPollAt - before
	require.InDelta(t, 120, delta, 5, "next_poll_at must use Retry-After (≈120s), not exponential backoff (1h)")
}

// TestWorker_NonCrawlerFeedRewritesMediaURLs verifies the Plan 15
// universal pass: a feed with crawler=0 (the default) whose entries
// arrive with raw <img> tags should still have those URLs rewritten
// through the proxy encoder before storage. Right-click→copy-image-URL
// must yield a /api/v1/proxy/<token> URL on summary-only feeds, not
// just on crawler-extracted feeds.
func TestWorker_NonCrawlerFeedRewritesMediaURLs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	t.Cleanup(func() { d.Close() })
	store := storage.New(d)

	// Feed-supplied content fragment with a relative <img> src — the
	// kind of payload non-crawler feeds produce. We assert the stored
	// content has the proxy-encoded src.
	feedXML := `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Summary feed</title>
  <id>https://t/</id>
  <entry>
    <id>e1</id>
    <title>Has image</title>
    <link href="https://t/post"/>
    <published>2026-04-26T08:00:00Z</published>
    <content type="html">&lt;p&gt;hello&lt;/p&gt;&lt;img src="/img/x.png"&gt;</content>
  </entry>
</feed>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(feedXML))
	}))
	t.Cleanup(srv.Close)

	pipeline := reader.NewPipeline(reader.PipelineConfig{
		Encode: func(u string) string { return "/api/v1/proxy/" + u },
	})

	worker := poller.NewWorker(poller.WorkerConfig{
		Store:      store,
		Client:     httpclient.NewClient(httpclient.Config{Timeout: 2 * time.Second, MaxBodyBytes: 1 << 20, AllowPrivate: true}),
		Limiter:    limiter.NewHostLimiter(1),
		Pipeline:   pipeline,
		PollFactor: 1.0,
	})

	feedID, err := store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "Summary", FeedURL: srv.URL, PollInterval: 3600,
		// Crawler is false (the default) — Plan 15 contract says even
		// these flow through RewriteAndSanitize.
	})
	require.NoError(t, err)

	require.NoError(t, worker.PollOne(context.Background(), feedID))

	entries, err := store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	stored, err := store.GetEntry(context.Background(), 1, entries[0].ID)
	require.NoError(t, err)
	require.NotNil(t, stored.Content, "content must be populated from feed payload")
	content := *stored.Content
	require.True(t, strings.Contains(content, "/api/v1/proxy/"),
		"non-crawler feeds must still get proxy-encoded media URLs; got %q", content)
	require.False(t, strings.Contains(content, `src="/img/x.png"`),
		"raw relative src must have been rewritten; got %q", content)
}

// TestWorker_CrawlerExtractionFailureSanitisesSummaryFallback verifies
// the security boundary added in Plan 15: when a crawler feed's
// extraction fails for an entry, the worker falls back to the
// feed-supplied summary. That fallback HTML never went through
// Pipeline.Process, so the universal sanitize pass MUST run on it
// before storage — otherwise a malicious feed could ship a <script>
// tag that the SPA renders via {@html entry.content}.
func TestWorker_CrawlerExtractionFailureSanitisesSummaryFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tap.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(context.Background(), d))
	t.Cleanup(func() { d.Close() })
	store := storage.New(d)

	// A summary containing a <script> tag and a relative <img>. The
	// crawler entry URL points at a server that 500s, so extraction
	// fails and the worker falls back to this summary.
	feedXML := `<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Bad summary feed</title>
  <id>https://t/</id>
  <entry>
    <id>e1</id>
    <title>XSS test</title>
    <link href="https://upstream.invalid/1"/>
    <published>2026-04-26T08:00:00Z</published>
    <summary type="html">&lt;p&gt;ok&lt;/p&gt;&lt;script&gt;alert(1)&lt;/script&gt;&lt;img src="/img/x.png"&gt;</summary>
  </entry>
</feed>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the feed; refuse extraction.
		if strings.HasPrefix(r.URL.Path, "/feed") || r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/atom+xml")
			_, _ = w.Write([]byte(feedXML))
			return
		}
		// Any article fetch: 500 to force extraction failure.
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	pipeline := reader.NewPipeline(reader.PipelineConfig{
		Encode: func(u string) string { return "/api/v1/proxy/" + u },
	})

	worker := poller.NewWorker(poller.WorkerConfig{
		Store:      store,
		Client:     httpclient.NewClient(httpclient.Config{Timeout: 2 * time.Second, MaxBodyBytes: 1 << 20, AllowPrivate: true}),
		Limiter:    limiter.NewHostLimiter(2),
		Pipeline:   pipeline,
		PollFactor: 1.0,
	})

	feedID, err := store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "BadSummary", FeedURL: srv.URL + "/feed",
		PollInterval: 3600,
		Crawler:      true, // crawler ON — extraction will fail and fall back
	})
	require.NoError(t, err)

	require.NoError(t, worker.PollOne(context.Background(), feedID))

	entries, err := store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
	require.NoError(t, err)
	require.Len(t, entries, 1)

	stored, err := store.GetEntry(context.Background(), 1, entries[0].ID)
	require.NoError(t, err)
	require.NotNil(t, stored.Content, "content must be populated from summary fallback")
	content := *stored.Content

	// Sanitiser must have stripped <script>.
	require.False(t, strings.Contains(content, "<script"),
		"crawler-fallback HTML must be sanitised; got %q", content)
	require.False(t, strings.Contains(content, "alert(1)"),
		"crawler-fallback HTML must be sanitised; got %q", content)
	// Media proxy must have rewritten relative <img>.
	require.True(t, strings.Contains(content, "/api/v1/proxy/"),
		"crawler-fallback HTML must have proxy-encoded media URLs; got %q", content)
}

func TestWorker_TombstonedHashNotReinserted(t *testing.T) {
	store, w, _ := setupWorkerTest(t)
	ctx := context.Background()
	feeds, _ := store.ListFeeds(ctx, 1)

	// First poll inserts the two entries.
	require.NoError(t, w.PollOne(ctx, feeds[0].ID))
	entries, _ := store.ListEntries(ctx, 1, storage.EntriesFilter{Limit: 10})
	require.Len(t, entries, 2)

	// Tombstone one of them. Later polls must not re-insert.
	target := entries[0]
	require.NoError(t, store.InsertTombstone(ctx, target.FeedID, target.Hash))
	// Hard-delete the row to simulate the archival path.
	_, err := store.DB().ExecContext(ctx, `DELETE FROM entries WHERE id = ?`, target.ID)
	require.NoError(t, err)

	require.NoError(t, w.PollOne(ctx, feeds[0].ID))
	entries, _ = store.ListEntries(ctx, 1, storage.EntriesFilter{Limit: 10})
	require.Len(t, entries, 1, "tombstone blocks re-insert")
}
