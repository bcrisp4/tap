package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func TestFeeds_Subscribe_AndList(t *testing.T) {
	f := newAPIFixture(t)

	w := f.do(t, "POST", "/api/v1/feeds",
		`{"feed_url":"https://jvns.ca/atom.xml","title":"jvns.ca"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	var sub struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sub))
	require.Greater(t, sub.ID, int64(0))

	w = f.do(t, "GET", "/api/v1/feeds", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), "jvns.ca")
	require.Contains(t, w.Body.String(), `"data"`)
	require.Contains(t, w.Body.String(), `"pagination"`)
}

func TestFeeds_Subscribe_SchedulesImmediatePoll(t *testing.T) {
	// design §6: POST /feeds schedules an immediate poll. Verify
	// next_poll_at is set to a value <= now.
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds",
		`{"feed_url":"https://jvns.ca/atom.xml"}`)
	require.Equal(t, http.StatusCreated, w.Code)

	feeds, err := f.store.ListFeeds(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, feeds, 1)
	require.NotNil(t, feeds[0].NextPollAt)
	require.LessOrEqual(t, *feeds[0].NextPollAt, time.Now().Unix())
}

func TestFeeds_Subscribe_DefaultsTitleToURL(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds",
		`{"feed_url":"https://jvns.ca/atom.xml"}`)
	require.Equal(t, http.StatusCreated, w.Code)
	feeds, _ := f.store.ListFeeds(context.Background(), 1)
	require.Equal(t, "https://jvns.ca/atom.xml", feeds[0].Title)
}

func TestFeeds_Subscribe_RequiresFeedURL(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds", `{"title":"oops"}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code":"missing_feed_url"`)
}

func TestFeeds_Subscribe_BadJSON(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds", `not json`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code":"bad_json"`)
}

func TestFeeds_Subscribe_DuplicateConflict(t *testing.T) {
	f := newAPIFixture(t)
	body := `{"feed_url":"https://jvns.ca/atom.xml","title":"jvns.ca"}`
	w := f.do(t, "POST", "/api/v1/feeds", body)
	require.Equal(t, http.StatusCreated, w.Code)
	w = f.do(t, "POST", "/api/v1/feeds", body)
	require.Equal(t, http.StatusConflict, w.Code)
	require.Contains(t, w.Body.String(), `"code":"duplicate"`)
}

func TestFeeds_GetMissing404(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "GET", "/api/v1/feeds/9999", "")
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), `"code":"not_found"`)
}

func TestFeeds_Update(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds",
		`{"feed_url":"https://x/","title":"X"}`)
	var sub struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sub))
	id := strconv.FormatInt(sub.ID, 10)

	w = f.do(t, "PUT", "/api/v1/feeds/"+id,
		`{"title":"Renamed","crawler":true}`)
	require.Equal(t, http.StatusOK, w.Code)

	w = f.do(t, "GET", "/api/v1/feeds/"+id, "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"title":"Renamed"`)
	require.Contains(t, w.Body.String(), `"crawler":true`)
}

func TestFeeds_Delete(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds",
		`{"feed_url":"https://x/","title":"X"}`)
	var sub struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &sub)
	id := strconv.FormatInt(sub.ID, 10)

	w = f.do(t, "DELETE", "/api/v1/feeds/"+id, "")
	require.Equal(t, http.StatusNoContent, w.Code)

	w = f.do(t, "GET", "/api/v1/feeds/"+id, "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestFeeds_Refresh_SetsNextPollNow(t *testing.T) {
	f := newAPIFixture(t)
	// Create a feed with next_poll_at far in the future so the
	// refresh has something to override.
	future := time.Now().Add(24 * time.Hour).Unix()
	feed := &storage.Feed{
		UserID:       1,
		Title:        "X",
		FeedURL:      "https://x/",
		PollInterval: 3600,
		NextPollAt:   &future,
	}
	id, err := f.store.CreateFeed(context.Background(), feed)
	require.NoError(t, err)

	w := f.do(t, "POST", "/api/v1/feeds/"+strconv.FormatInt(id, 10)+"/refresh", "")
	require.Equal(t, http.StatusAccepted, w.Code)

	got, err := f.store.GetFeed(context.Background(), 1, id)
	require.NoError(t, err)
	require.NotNil(t, got.NextPollAt)
	require.LessOrEqual(t, *got.NextPollAt, time.Now().Unix())
}

func TestFeeds_Refresh_BadID(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds/notnum/refresh", "")
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFeeds_Discover(t *testing.T) {
	// Stand up a tiny HTML page exposing two <link rel="alternate">.
	// AllowPrivate=true on the test client lets us hit httptest.
	srv := startDiscoverServer(t, `<!doctype html><html><head>
		<link rel="alternate" type="application/rss+xml" title="Main RSS" href="/feed.xml">
		<link rel="alternate" type="application/atom+xml" href="https://example.com/atom.xml">
		<link rel="stylesheet" href="/site.css">
	</head><body></body></html>`)

	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds/discover",
		`{"url":"`+srv.URL+`"}`)
	require.Equal(t, http.StatusOK, w.Code)

	var got struct {
		Candidates []struct {
			HRef  string `json:"href"`
			Title string `json:"title"`
			Type  string `json:"type"`
		} `json:"candidates"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Candidates, 2)
	require.Equal(t, srv.URL+"/feed.xml", got.Candidates[0].HRef)
	require.Equal(t, "Main RSS", got.Candidates[0].Title)
	require.Equal(t, "https://example.com/atom.xml", got.Candidates[1].HRef)
}

func TestFeeds_Discover_BadJSON(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "POST", "/api/v1/feeds/discover", `{}`)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code":"bad_json"`)
}
