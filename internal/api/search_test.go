package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/storage"
)

func TestSearch_FTS5(t *testing.T) {
	f := newAPIFixture(t)
	feedID, err := f.store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "T", FeedURL: "https://t/", PollInterval: 3600,
	})
	require.NoError(t, err)

	body := "the quick brown fox jumps"
	for _, e := range []*storage.Entry{
		{FeedID: feedID, UserID: 1, Hash: "a", Title: "About foxes", Content: &body},
		{FeedID: feedID, UserID: 1, Hash: "b", Title: "Cooking with apples"},
	} {
		_, err := f.store.InsertEntry(context.Background(), e)
		require.NoError(t, err)
	}

	w := f.do(t, "GET", "/api/v1/search?q="+url.QueryEscape("fox"), "")
	require.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]int   `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Data, 1)
	require.Contains(t, got.Data[0]["title"], "foxes")
}

func TestSearch_MissingQuery(t *testing.T) {
	f := newAPIFixture(t)
	w := f.do(t, "GET", "/api/v1/search", "")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), `"code":"missing_query"`)
}

func TestSearch_PaginationTotalCountsAllMatches(t *testing.T) {
	// Regression: previously total = len(page) which broke pagination
	// for queries with more matches than `limit`.
	f := newAPIFixture(t)
	feedID, _ := f.store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "T", FeedURL: "https://t/", PollInterval: 3600,
	})
	for i, h := range []string{"a", "b", "c", "d", "e"} {
		body := "kingfisher result " + h
		_, err := f.store.InsertEntry(context.Background(), &storage.Entry{
			FeedID: feedID, UserID: 1, Hash: h,
			Title: "post " + h, Content: &body,
		})
		require.NoError(t, err)
		_ = i
	}

	w := f.do(t, "GET", "/api/v1/search?q=kingfisher&limit=2", "")
	require.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Data       []map[string]any `json:"data"`
		Pagination map[string]int   `json:"pagination"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Data, 2)
	require.Equal(t, 5, got.Pagination["total"])
	require.Equal(t, 2, got.Pagination["limit"])
}

func TestSearch_StripsContentFromList(t *testing.T) {
	f := newAPIFixture(t)
	feedID, _ := f.store.CreateFeed(context.Background(), &storage.Feed{
		UserID: 1, Title: "T", FeedURL: "https://t/", PollInterval: 3600,
	})
	body := "lorem ipsum kingfisher dolor"
	_, _ = f.store.InsertEntry(context.Background(), &storage.Entry{
		FeedID: feedID, UserID: 1, Hash: "a", Title: "Birds", Content: &body,
	})
	w := f.do(t, "GET", "/api/v1/search?q=kingfisher", "")
	require.Equal(t, http.StatusOK, w.Code)
	var got struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Data, 1)
	_, hasContent := got.Data[0]["content"]
	require.False(t, hasContent, "content excluded from search list payload")
}
