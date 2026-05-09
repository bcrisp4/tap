package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpx"
	"github.com/bcrisp4/tap/internal/poll"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/stretchr/testify/require"
)

func TestEndToEnd_SubscribePollServeEntries(t *testing.T) {
	t.Parallel()

	const atom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>End-to-end</title>
  <id>urn:e2e</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>Hello</title>
    <id>urn:e2e:1</id>
    <link href="https://e2e.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;hello&lt;/p&gt;&lt;script&gt;alert(1)&lt;/script&gt;&lt;a href=&quot;https://e.com/?utm_source=feed&amp;id=1&quot; onclick=&quot;evil()&quot;&gt;link&lt;/a&gt;&lt;iframe src=&quot;https://evil.example/embed&quot;&gt;&lt;/iframe&gt;&lt;img src=&quot;https://t.example/pixel&quot; width=&quot;1&quot; height=&quot;1&quot;&gt;</content>
  </entry>
</feed>`
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atom))
	}))
	t.Cleanup(feedSrv.Close)

	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))

	mux := api.NewMux(d, api.MuxOpts{})

	// POST /api/v1/subscriptions
	body := strings.NewReader(`{"feed_url":"` + feedSrv.URL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Drive the scheduler.
	sched := poll.NewScheduler(context.Background(), d, http.DefaultClient, poll.SchedulerOpts{
		Workers:   1,
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
	})
	defer sched.Stop()
	sched.Tick(context.Background())
	require.NoError(t, sched.Wait(5*time.Second))

	// GET /api/v1/entries
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	require.Equal(t, http.StatusOK, rr2.Code)

	var resp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "Hello", resp.Data[0]["title"])

	// Detail GET — body must be sanitised.
	entryID := int64(resp.Data[0]["id"].(float64))
	detailURL := "/api/v1/entries/" + strconv.FormatInt(entryID, 10)
	rr3 := httptest.NewRecorder()
	mux.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, detailURL, nil))
	require.Equal(t, http.StatusOK, rr3.Code, rr3.Body.String())

	var detail struct {
		Content string `json:"content"`
	}
	require.NoError(t, json.NewDecoder(rr3.Body).Decode(&detail))
	require.NotContains(t, detail.Content, "<script>")
	require.NotContains(t, detail.Content, "alert")
	require.NotContains(t, detail.Content, "onclick")
	require.NotContains(t, detail.Content, "utm_source")
	require.NotContains(t, detail.Content, "<iframe")
	require.NotContains(t, detail.Content, "t.example/pixel")
	require.Contains(t, detail.Content, "<p>hello</p>")
}

func TestEndToEnd_ProxyURLsRewriteAndServe(t *testing.T) {
	t.Parallel()

	// Origin server: returns a PNG fixture for any path.
	pngFixture := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	}
	var imageHits int32
	imageSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&imageHits, 1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngFixture)
	}))
	t.Cleanup(imageSrv.Close)

	imageURL := imageSrv.URL + "/img.png"
	atomFeed := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Imgs</title>
  <id>urn:imgs</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>HasImg</title>
    <id>urn:imgs:1</id>
    <link href="https://example.com/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;hi&lt;/p&gt;&lt;img src=&quot;` + imageURL + `&quot;&gt;</content>
  </entry>
</feed>`
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(atomFeed))
	}))
	t.Cleanup(feedSrv.Close)

	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))

	// Generate a signing key and insert into configuration (simulating bootstrap).
	key := []byte("0123456789abcdef0123456789abcdef")
	_, err = db.SetConfigIfAbsent(context.Background(), d, proxySigningKeyConfigKey, key)
	require.NoError(t, err)

	signer := proxy.NewSigner(key)
	cache := proxy.NewCache(t.TempDir(), 1<<20)
	proxyHandler := proxy.NewHandler(signer, cache, http.DefaultClient, 10<<20)

	mux := api.NewMux(d, api.MuxOpts{ProxyHandler: proxyHandler})

	// Subscribe.
	body := strings.NewReader(`{"feed_url":"` + feedSrv.URL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Drive a poll with the production-shaped processor (rewriter wired in).
	proc := processor.New(sanitise.DefaultPolicy(), signer.RewriteImageURL)
	sched := poll.NewScheduler(context.Background(), d, http.DefaultClient, poll.SchedulerOpts{
		Workers:   1,
		Processor: proc,
	})
	defer sched.Stop()
	sched.Tick(context.Background())
	require.NoError(t, sched.Wait(5*time.Second))

	// Fetch the entry list, then the entry detail; the body must contain a proxy URL.
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	require.Equal(t, http.StatusOK, rr2.Code)
	var listResp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&listResp))
	require.Len(t, listResp.Data, 1)

	entryID := int64(listResp.Data[0]["id"].(float64))
	rr3 := httptest.NewRecorder()
	mux.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/v1/entries/"+strconv.FormatInt(entryID, 10), nil))
	require.Equal(t, http.StatusOK, rr3.Code, rr3.Body.String())
	var detail struct {
		Content string `json:"content"`
	}
	require.NoError(t, json.NewDecoder(rr3.Body).Decode(&detail))
	require.Contains(t, detail.Content, `src="/api/v1/proxy/`, "img src must be rewritten: %s", detail.Content)
	require.NotContains(t, detail.Content, imageURL, "raw origin URL must not appear: %s", detail.Content)

	// Extract the proxy URL from the body and GET it.
	const marker = `src="/api/v1/proxy/`
	idx := strings.Index(detail.Content, marker)
	require.GreaterOrEqual(t, idx, 0)
	end := strings.Index(detail.Content[idx+len(marker):], `"`)
	require.GreaterOrEqual(t, end, 0)
	proxyURL := "/api/v1/proxy/" + detail.Content[idx+len(marker):idx+len(marker)+end]

	rr4 := httptest.NewRecorder()
	mux.ServeHTTP(rr4, httptest.NewRequest(http.MethodGet, proxyURL, nil))
	require.Equal(t, http.StatusOK, rr4.Code, rr4.Body.String())
	require.Equal(t, "image/png", rr4.Header().Get("Content-Type"))
	require.Equal(t, "public, max-age=31536000, immutable", rr4.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", rr4.Header().Get("X-Content-Type-Options"))
	require.Equal(t, pngFixture, rr4.Body.Bytes())
	require.Equal(t, int32(1), atomic.LoadInt32(&imageHits))

	// Second request: served from cache, no new origin hit.
	rr5 := httptest.NewRecorder()
	mux.ServeHTTP(rr5, httptest.NewRequest(http.MethodGet, proxyURL, nil))
	require.Equal(t, http.StatusOK, rr5.Code)
	require.Equal(t, int32(1), atomic.LoadInt32(&imageHits), "second request must hit cache")

	// Restart simulation: reload the signing key from the DB and rebuild the
	// signer + handler. The previously-issued token must still verify.
	keyAgain, ok, err := db.GetConfig(context.Background(), d, proxySigningKeyConfigKey)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, key, keyAgain)

	signer2 := proxy.NewSigner(keyAgain)
	tok := strings.TrimPrefix(proxyURL, "/api/v1/proxy/")
	gotURL, ok := signer2.Verify(tok)
	require.True(t, ok)
	require.Equal(t, imageURL, gotURL)
}

func TestE2E_SSRFRejectsLoopbackByDefault(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmp := t.TempDir()
	d, err := db.Open(ctx, filepath.Join(tmp, "tap.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	client := httpx.NewClient(httpx.Opts{Timeout: 5 * time.Second, SSRF: httpx.SSRFPolicy{}})
	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Workers:   1,
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
	})
	sched.Start()
	t.Cleanup(sched.Stop)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "Loopback", FeedURL: srv.URL, NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	sched.Tick(ctx)
	require.NoError(t, sched.Wait(2*time.Second))

	var lastError sql.NullString
	require.NoError(t, d.QueryRowContext(ctx, `SELECT last_error FROM subscriptions WHERE id=?`, subID).Scan(&lastError))
	require.True(t, lastError.Valid, "expected last_error to be set")
	require.Contains(t, lastError.String, "ssrf", "expected ssrf in error: %s", lastError.String)
}

func TestE2E_SSRFAllowlistAcceptsLoopback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tmp := t.TempDir()
	d, err := db.Open(ctx, filepath.Join(tmp, "tap.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	policy, err := httpx.ParseSSRFPolicy(false, []string{"127.0.0.0/8"})
	require.NoError(t, err)
	client := httpx.NewClient(httpx.Opts{Timeout: 5 * time.Second, SSRF: policy})
	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
		Processor: processor.New(sanitise.DefaultPolicy(), nil),
		Workers:   1,
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
	})
	sched.Start()
	t.Cleanup(sched.Stop)

	subID, err := db.InsertSubscription(ctx, d, db.NewSubscription{
		Title: "Allowed", FeedURL: srv.URL, NextPoll: 0, Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	sched.Tick(ctx)
	require.NoError(t, sched.Wait(2*time.Second))

	var lastError sql.NullString
	require.NoError(t, d.QueryRowContext(ctx, `SELECT last_error FROM subscriptions WHERE id=?`, subID).Scan(&lastError))
	require.False(t, lastError.Valid, "expected no error, got %q", lastError.String)
}
