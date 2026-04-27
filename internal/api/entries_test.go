package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func seedEntries(t *testing.T, f *apiFixture) (feedID int64) {
	t.Helper()
	id, err := f.store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "T", FeedURL: "https://t/", PollInterval: 3600,
	})
	require.NoError(t, err)
	body := "the quick brown fox"
	for _, h := range []string{"a", "b", "c"} {
		_, err := f.store.InsertEntry(context.Background(), &storage.Entry{
			FeedID: id, UserID: 1, Hash: h, Title: "Title " + h, Content: &body,
		})
		require.NoError(t, err)
	}
	return id
}

func TestEntries_ListUnreadDefault(t *testing.T) {
	f := newAPIFixture(t)
	seedEntries(t, f)
	w := f.do(t, "GET", "/api/v1/entries", "")
	require.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]int   `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Data, 3)
	require.Equal(t, 3, got.Pagination["total"])
	require.Equal(t, 50, got.Pagination["limit"])
	for _, e := range got.Data {
		_, hasContent := e["content"]
		require.False(t, hasContent, "content excluded from list payloads")
	}
}

func TestEntries_List_FiltersByFeed(t *testing.T) {
	f := newAPIFixture(t)
	feedID := seedEntries(t, f)
	// A second feed whose entries should be filtered out.
	other, _ := f.store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "Other", FeedURL: "https://o/", PollInterval: 3600,
	})
	_, _ = f.store.InsertEntry(context.Background(), &storage.Entry{
		FeedID: other, UserID: 1, Hash: "x", Title: "Other entry",
	})

	w := f.do(t, "GET", "/api/v1/entries?feed_id="+strconv.FormatInt(feedID, 10), "")
	require.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Data, 3)
}

func TestEntries_List_StatusAll(t *testing.T) {
	f := newAPIFixture(t)
	seedEntries(t, f)
	// Mark one read, then list status=all and confirm we get 3.
	entries, _ := f.store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
	tr := true
	require.NoError(t, f.store.UpdateEntryState(context.Background(), 1, entries[0].ID, &tr, nil))

	w := f.do(t, "GET", "/api/v1/entries?status=all", "")
	require.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Data, 3)
}

func TestEntries_GetIncludesContentAndEnclosures(t *testing.T) {
	f := newAPIFixture(t)
	seedEntries(t, f)
	entries, _ := f.store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
	require.NotEmpty(t, entries)
	id := entries[0].ID
	_, err := f.store.InsertEnclosure(context.Background(), id, "https://t/x.mp3", "audio/mpeg", 100)
	require.NoError(t, err)

	w := f.do(t, "GET", "/api/v1/entries/"+strconv.FormatInt(id, 10), "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"enclosures"`)
	require.Contains(t, w.Body.String(), `"content"`)
	require.Contains(t, w.Body.String(), "x.mp3")
}

func TestEntries_Get_NotFound(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "GET", "/api/v1/entries/9999", "")
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestEntries_PutMarkRead(t *testing.T) {
	f := newAPIFixture(t)
	seedEntries(t, f)
	entries, _ := f.store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
	id := entries[0].ID

	w := f.do(t, "PUT", "/api/v1/entries/"+strconv.FormatInt(id, 10),
		`{"read":true}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"read":true`)

	got, _ := f.store.GetEntry(context.Background(), 1, id)
	require.True(t, got.Read)
}

func TestEntries_PutMarkSaved(t *testing.T) {
	f := newAPIFixture(t)
	seedEntries(t, f)
	entries, _ := f.store.ListEntries(context.Background(), 1, storage.EntriesFilter{Limit: 10})
	id := entries[0].ID

	w := f.do(t, "PUT", "/api/v1/entries/"+strconv.FormatInt(id, 10),
		`{"saved":true}`)
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"saved":true`)
}

func TestEntries_BulkRead_ByFeed(t *testing.T) {
	f := newAPIFixture(t)
	feedID := seedEntries(t, f)
	w := f.do(t, "PUT", "/api/v1/entries/read",
		`{"feed_id":`+strconv.FormatInt(feedID, 10)+`}`)
	require.Equal(t, http.StatusNoContent, w.Code)

	unread, _ := f.store.ListEntries(context.Background(), 1,
		storage.EntriesFilter{Status: "unread", Limit: 10})
	require.Empty(t, unread)
}

func TestEntries_BulkRead_All(t *testing.T) {
	f := newAPIFixture(t)
	seedEntries(t, f)
	w := f.do(t, "PUT", "/api/v1/entries/read", `{}`)
	require.Equal(t, http.StatusNoContent, w.Code)

	unread, _ := f.store.ListEntries(context.Background(), 1,
		storage.EntriesFilter{Status: "unread", Limit: 10})
	require.Empty(t, unread)
}
