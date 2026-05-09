package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/poll"
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
	defer feedSrv.Close()

	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(context.Background(), d))

	mux := api.NewMux(d, nil)

	// POST /api/v1/subscriptions
	body := strings.NewReader(`{"feed_url":"` + feedSrv.URL + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Drive the scheduler.
	sched := poll.NewScheduler(context.Background(), d, http.DefaultClient, poll.SchedulerOpts{
		Workers: 1,
		Cadence: time.Hour,
		Policy:  sanitise.DefaultPolicy(),
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
