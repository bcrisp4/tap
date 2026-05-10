package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/stretchr/testify/require"
)

func postSubscription(t *testing.T, mux http.Handler, body string) int64 {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var got struct {
		ID int64 `json:"id"`
	}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	return got.ID
}

func newAPI(t *testing.T) (*http.ServeMux, *sql.DB) {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	return NewMux(d, MuxOpts{}), d
}

func TestSubscriptions_PostThenList(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body, _ := json.Marshal(map[string]string{"feed_url": "https://example.com/feed"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil))
	require.Equal(t, http.StatusOK, rr2.Code)

	var resp struct {
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.NewDecoder(rr2.Body).Decode(&resp))
	require.Len(t, resp.Data, 1)
	require.Equal(t, "https://example.com/feed", resp.Data[0]["feed_url"])
}

func TestSubscriptions_PostRejectsBadURL(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body, _ := json.Marshal(map[string]string{"feed_url": "not-a-url"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPostSubscriptions_PersistsExtractFlag(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body := strings.NewReader(`{"feed_url":"https://x.example/feed","extract":true}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, true, got["extract"])
	require.Equal(t, "", got["extract_selector"])
}

func TestPostSubscriptions_DefaultsExtractFalse(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body := strings.NewReader(`{"feed_url":"https://x.example/feed"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	require.Equal(t, false, got["extract"])
}

func TestPatchSubscription_TogglesExtract(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	body := strings.NewReader(`{"extract":true}`)
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10), body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var out map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&out))
	require.Equal(t, true, out["extract"])
}

func TestPatchSubscription_SetsAndClearsSelector(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)
	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	patch := func(body string) map[string]any {
		req := httptest.NewRequest(http.MethodPatch,
			"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10),
			strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var out map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&out))
		return out
	}

	got := patch(`{"extract_selector":".article-body"}`)
	require.Equal(t, ".article-body", got["extract_selector"])

	got = patch(`{"extract_selector":""}`)
	require.Equal(t, "", got["extract_selector"])
}

func TestPatchSubscription_MalformedSelectorReturns400(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)
	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	body := strings.NewReader(`{"extract_selector":"[unclosed"}`)
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10), body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), `"code":"extract_selector_invalid"`)

	// Confirm the DB row is unchanged.
	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	require.Equal(t, http.StatusOK, getRR.Code)
	require.Contains(t, getRR.Body.String(), `"extract_selector":""`)
}

func TestPatchSubscription_UnknownIDReturns404(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)

	body := strings.NewReader(`{"extract":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/9999", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

func TestPatchSubscription_MalformedJSONReturns400(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)
	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10),
		strings.NewReader(`{not json`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}

func TestPatchSubscription_PartialUpdate_ExtractOnlyKeepsSelector(t *testing.T) {
	t.Parallel()
	mux, _ := newAPI(t)
	subID := postSubscription(t, mux, `{"feed_url":"https://x.example/feed"}`)

	patch := func(body string) {
		req := httptest.NewRequest(http.MethodPatch,
			"/api/v1/subscriptions/"+strconv.FormatInt(subID, 10),
			strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	}
	patch(`{"extract_selector":".article"}`)

	// Toggle extract; selector must NOT be cleared.
	patch(`{"extract":true}`)

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	getRR := httptest.NewRecorder()
	mux.ServeHTTP(getRR, getReq)
	require.Contains(t, getRR.Body.String(), `"extract_selector":".article"`)
	require.Contains(t, getRR.Body.String(), `"extract":true`)
}
