package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func TestEntries_ListAndPatchRead(t *testing.T) {
	t.Parallel()
	mux, d, uid := newAPIWithUser(t)

	subID, _ := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://example.com/feed", NextPoll: 0, Created: 0,
	})
	_, err := db.UpdateAfterPoll(context.Background(), d, subID, db.PollResult{
		UserID: uid, NowUnix: 100, Floor: 15 * time.Minute, Ceiling: 24 * time.Hour, NewEntries: []db.NewEntry{
			{Hash: "h1", Title: "A", URL: "https://e.com/a", Content: "<p>a</p>", PublishedAt: 50},
		},
	})
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/entries?unread=1", nil))
	require.Equal(t, http.StatusOK, rr.Code)

	var listResp struct {
		Data       []map[string]any `json:"data"`
		NextCursor *string          `json:"next_cursor"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&listResp))
	require.Len(t, listResp.Data, 1)

	id := int64(listResp.Data[0]["id"].(float64))

	patch, _ := json.Marshal(map[string]bool{"read": true})
	pr := httptest.NewRequest(http.MethodPatch, "/api/v1/entries/"+toStr(id), bytes.NewReader(patch))
	pr.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, pr)
	require.Equal(t, http.StatusOK, rr2.Code, rr2.Body.String())

	rr3 := httptest.NewRecorder()
	mux.ServeHTTP(rr3, httptest.NewRequest(http.MethodGet, "/api/v1/entries?unread=1", nil))
	var after struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr3.Body).Decode(&after))
	require.Len(t, after.Data, 0, "marking read should remove from unread list")
}

func toStr(i int64) string {
	return strconv.FormatInt(i, 10)
}

func TestEntries_ListIncludesExtractFailed(t *testing.T) {
	t.Parallel()
	mux, d, uid := newAPIWithUser(t)

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(), `
		INSERT INTO entries (user_id, subscription_id, hash, title, url, content, published_at, fetched_at, extract_failed)
		VALUES (?, ?, 'h', 'T', 'https://x/1', '<p>x</p>', 0, 0, 1)
	`, uid, subID)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/entries", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"extract_failed":true`)
}

func TestGetEntry_IncludesExtractFailed(t *testing.T) {
	t.Parallel()
	mux, d, uid := newAPIWithUser(t)

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	res, err := d.ExecContext(context.Background(), `
		INSERT INTO entries (user_id, subscription_id, hash, title, url, content, published_at, fetched_at, extract_failed)
		VALUES (?, ?, 'h', 'T', 'https://x/1', '<p>x</p>', 0, 0, 1)
	`, uid, subID)
	require.NoError(t, err)
	entryID, err := res.LastInsertId()
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/entries/"+toStr(entryID), nil))
	require.Equal(t, http.StatusOK, rr.Code)
	require.Contains(t, rr.Body.String(), `"extract_failed":true`)
}
