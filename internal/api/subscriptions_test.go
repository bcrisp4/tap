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

// newSubscriptionsTestMux builds an unauthenticated subscriptions mux for unit
// tests. The session-middleware tests live in middleware_test.go; tests here
// focus on handler logic in isolation, so the mux is constructed without auth
// wrapping. End-to-end auth coverage lives in cmd/tap/main_test.go.
func newSubscriptionsTestMux(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	d := newTestDB(t)
	m := http.NewServeMux()
	registerSubscriptionRoutes(m, d, nil)
	return m, d
}

func TestPostSubscriptionAcceptsCredentialsAndGetReturnsBooleans(t *testing.T) {
	t.Parallel()
	mux, _ := newSubscriptionsTestMux(t)

	body := map[string]any{
		"feed_url":        "https://x.example/feed",
		"basic_auth_user": "ben",
		"basic_auth_pass": "secret",
		"cookie":          "session=abc",
	}
	bs, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(bs))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	// Response should contain has_cookie + has_basic_auth, but NOT the values.
	respBody := rr.Body.String()
	require.Contains(t, respBody, `"has_cookie":true`)
	require.Contains(t, respBody, `"has_basic_auth":true`)
	require.NotContains(t, respBody, "secret")
	require.NotContains(t, respBody, `"cookie":"session=abc"`)
	require.NotContains(t, respBody, "basic_auth_user")
	require.NotContains(t, respBody, "basic_auth_pass")

	// GET the list — same expectations.
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/subscriptions", nil)
	listRR := httptest.NewRecorder()
	mux.ServeHTTP(listRR, listReq)
	require.Equal(t, http.StatusOK, listRR.Code)
	listBody := listRR.Body.String()
	require.Contains(t, listBody, `"has_cookie":true`)
	require.Contains(t, listBody, `"has_basic_auth":true`)
	require.NotContains(t, listBody, "secret")
}

func TestPostSubscriptionWithoutCredentialsReturnsBooleanFalse(t *testing.T) {
	t.Parallel()
	mux, _ := newSubscriptionsTestMux(t)

	body := map[string]any{"feed_url": "https://x.example/feed"}
	bs, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(bs))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	require.Contains(t, rr.Body.String(), `"has_cookie":false`)
	require.Contains(t, rr.Body.String(), `"has_basic_auth":false`)
}

func TestPatchSubscriptionCredentialMergeSemantics(t *testing.T) {
	t.Parallel()
	mux, d := newSubscriptionsTestMux(t)

	// Seed a subscription with all three creds set.
	id, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.NoError(t, err)

	// Step A: PATCH without any credential field — no change.
	patchSubscription(mux, t, id, `{}`)
	got, _ := db.GetSubscription(context.Background(), d, id)
	require.Equal(t, "c", got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "p", got.BasicAuthPass)

	// Step B: PATCH cookie="" — clear cookie only.
	patchSubscription(mux, t, id, `{"cookie":""}`)
	got, _ = db.GetSubscription(context.Background(), d, id)
	require.Empty(t, got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "p", got.BasicAuthPass)

	// Step C: PATCH basic_auth_pass set to a new value.
	patchSubscription(mux, t, id, `{"basic_auth_pass":"newpass"}`)
	got, _ = db.GetSubscription(context.Background(), d, id)
	require.Empty(t, got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "newpass", got.BasicAuthPass)

	// Step D: PATCH extract:true — does NOT clobber any credential.
	patchSubscription(mux, t, id, `{"extract":true}`)
	got, _ = db.GetSubscription(context.Background(), d, id)
	require.True(t, got.Extract)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "newpass", got.BasicAuthPass)

	// Step E: PATCH cookie:"x" — does NOT clobber extract or basic_auth_*.
	patchSubscription(mux, t, id, `{"cookie":"x"}`)
	got, _ = db.GetSubscription(context.Background(), d, id)
	require.True(t, got.Extract)
	require.Equal(t, "x", got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "newpass", got.BasicAuthPass)
}

// patchSubscription is a tiny helper for the test above.
func patchSubscription(mux http.Handler, t *testing.T, id int64, body string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(id, 10), strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code, "PATCH body=%s body=%s", body, rr.Body.String())
}
