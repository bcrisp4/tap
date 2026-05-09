package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/poll"
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
    <content type="html">&lt;p&gt;hello&lt;/p&gt;</content>
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
	sched := poll.NewScheduler(context.Background(), d, http.DefaultClient, poll.SchedulerOpts{Workers: 1, Cadence: time.Hour})
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
}
