package poll

import (
	"context"
	"database/sql"
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
		Cadence: 30 * time.Minute,
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
		Cadence: 30 * time.Minute,
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
		Cadence: 30 * time.Minute,
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
