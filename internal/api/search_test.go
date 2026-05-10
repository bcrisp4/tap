package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
)

func TestSearchAPI_QueryTooShort(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertAPITestUser(t, d, "search_short")
	mux := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: userID, Username: "search_short", Role: "admin"}})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/search?q=ab", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp ErrorEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, ErrCodeQueryTooShort, resp.Error.Code)
}

func TestSearchAPI_ValidQuery(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertAPITestUser(t, d, "search_valid")
	mux := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: userID, Username: "search_valid", Role: "admin"}})

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: userID, Title: "Feed", FeedURL: "https://sv.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'hs1', 'SearchableTitleXYZ', 'auth', 'https://sv.com/1', '<p>content</p>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), userID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/search?q=SearchableTitleXYZ", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Data []searchResultDTO `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "SearchableTitleXYZ", resp.Data[0].Title)
}

func TestSearchAPI_CrossUserIsolation(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertAPITestUser(t, d, "su1")
	u2 := insertAPITestUser(t, d, "su2")

	subID1, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: u1, Title: "Feed", FeedURL: "https://su1.com/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'hsu1', 'PrivateSearchTermABC', 'auth', 'https://su1.com/1', '<p>content</p>', ?, ?, 0, 0, ?)`,
		subID1, time.Now().Unix(), time.Now().Unix(), u1)
	require.NoError(t, err)

	mux2 := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: u2, Username: "su2", Role: "admin"}})
	w := httptest.NewRecorder()
	mux2.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/search?q=PrivateSearchTermABC", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct{ Data []searchResultDTO `json:"data"` }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Empty(t, resp.Data)
}

func TestSearchAPI_Unauthenticated(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	mux := NewTestMux(d, TestMuxOpts{})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/search?q=hello", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
