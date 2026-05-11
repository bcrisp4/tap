package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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

// newAPIWithUser creates an unauthenticated mux with a test user injected into
// context for all requests. Returns the mux, db, and the user ID.
func newAPIWithUser(t *testing.T) (http.Handler, *sql.DB, int64) {
	t.Helper()
	d := newTestDB(t)
	uid := seedUser(t, d, "testuser", "testpass", "admin")
	u, err := db.GetUserByID(context.Background(), d, uid)
	require.NoError(t, err)
	testSess := db.Session{ID: 1, UserID: uid, CSRFToken: "test-csrf"}

	m := http.NewServeMux()
	registerSubscriptionRoutes(m, d, nil)
	registerEntryRoutes(m, d)

	// Wrap each request to inject the test user and session into context.
	wrapped := withFakeAuth(t, u, testSess, m)
	return wrapped, d, uid
}

// newAPI returns an UNAUTHENTICATED mux for unit-test handlers — with a test
// user injected into context so handlers can resolve userFromContext.
func newAPI(t *testing.T) (*http.ServeMux, *sql.DB) {
	t.Helper()
	d, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(context.Background(), d))
	m := http.NewServeMux()
	registerSubscriptionRoutes(m, d, nil)
	registerEntryRoutes(m, d)
	return m, d
}

func TestSubscriptions_PostThenList(t *testing.T) {
	t.Parallel()
	mux, _, _ := newAPIWithUser(t)

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
	mux, _, _ := newAPIWithUser(t)

	body, _ := json.Marshal(map[string]string{"feed_url": "not-a-url"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestPostSubscriptions_PersistsExtractFlag(t *testing.T) {
	t.Parallel()
	mux, _, _ := newAPIWithUser(t)

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
	mux, _, _ := newAPIWithUser(t)

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
	mux, _, _ := newAPIWithUser(t)

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
	mux, _, _ := newAPIWithUser(t)
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
	mux, _, _ := newAPIWithUser(t)
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
	mux, _, _ := newAPIWithUser(t)

	body := strings.NewReader(`{"extract":true}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/subscriptions/9999", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

func TestPatchSubscription_MalformedJSONReturns400(t *testing.T) {
	t.Parallel()
	mux, _, _ := newAPIWithUser(t)
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
	mux, _, _ := newAPIWithUser(t)
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

// newSubscriptionsTestMux builds an unauthenticated subscriptions mux with
// a test user injected for unit tests.
func newSubscriptionsTestMux(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	mux, d, _ := newAPIWithUser(t)
	return mux, d
}

func TestPostSubscriptionAcceptsCredentialsAndGetReturnsBooleans(t *testing.T) {
	t.Parallel()
	mux, _, _ := newAPIWithUser(t)

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
	mux, _, _ := newAPIWithUser(t)

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
	mux, d, uid := newAPIWithUser(t)

	// Seed a subscription with all three creds set.
	id, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed", NextPoll: 0, Created: 0,
		Cookie: "c", BasicAuthUser: "u", BasicAuthPass: "p",
	})
	require.NoError(t, err)

	// Step A: PATCH without any credential field — no change.
	patchSubscription(mux, t, id, `{}`)
	got, _ := db.GetSubscription(context.Background(), d, id, uid)
	require.Equal(t, "c", got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "p", got.BasicAuthPass)

	// Step B: PATCH cookie="" — clear cookie only.
	patchSubscription(mux, t, id, `{"cookie":""}`)
	got, _ = db.GetSubscription(context.Background(), d, id, uid)
	require.Empty(t, got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "p", got.BasicAuthPass)

	// Step C: PATCH basic_auth_pass set to a new value.
	patchSubscription(mux, t, id, `{"basic_auth_pass":"newpass"}`)
	got, _ = db.GetSubscription(context.Background(), d, id, uid)
	require.Empty(t, got.Cookie)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "newpass", got.BasicAuthPass)

	// Step D: PATCH extract:true — does NOT clobber any credential.
	patchSubscription(mux, t, id, `{"extract":true}`)
	got, _ = db.GetSubscription(context.Background(), d, id, uid)
	require.True(t, got.Extract)
	require.Equal(t, "u", got.BasicAuthUser)
	require.Equal(t, "newpass", got.BasicAuthPass)

	// Step E: PATCH cookie:"x" — does NOT clobber extract or basic_auth_*.
	patchSubscription(mux, t, id, `{"cookie":"x"}`)
	got, _ = db.GetSubscription(context.Background(), d, id, uid)
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

func TestCreateSubscription_BodyTooLarge(t *testing.T) {
	t.Parallel()
	mux, _, _ := newAPIWithUser(t)

	// Body is valid JSON for the first >1 MiB, then keeps going — triggers MaxBytesError, not syntax error.
	body := `{"feed_url":"` + strings.Repeat("a", 2<<20) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, rr.Code, rr.Body.String())
}

func TestPatchSubscription_BodyTooLarge(t *testing.T) {
	t.Parallel()
	mux, d, uid := newAPIWithUser(t)
	id, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid, Title: "x", FeedURL: "https://x.example/feed",
	})
	require.NoError(t, err)

	body := `{"feed_url":"` + strings.Repeat("a", 2<<20) + `"}`
	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(id, 10), strings.NewReader(body))
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, rr.Code, rr.Body.String())
}

func TestSubscriptionsAPI_MarkRead_Happy(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertAPITestUser(t, d, "subuser-mr")
	user := db.User{ID: userID, Username: "subuser-mr", Role: "admin"}
	mux := NewTestMux(d, TestMuxOpts{TestUser: user})

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: userID, Title: "F", FeedURL: "https://x/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(),
		`INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at, read, saved, user_id)
		 VALUES (?, 'h', 'E', '', 'https://x/1', '<p>x</p>', ?, ?, 0, 0, ?)`,
		subID, time.Now().Unix(), time.Now().Unix(), userID)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST",
		fmt.Sprintf("/api/v1/subscriptions/%d/mark-read", subID), nil))
	require.Equal(t, http.StatusNoContent, w.Code)

	var n int
	require.NoError(t, d.QueryRowContext(context.Background(),
		`SELECT read FROM entries WHERE subscription_id = ?`, subID).Scan(&n))
	require.Equal(t, 1, n)
}

func TestSubscriptionsAPI_MarkRead_OtherUserGets404(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	u1 := insertAPITestUser(t, d, "mr-u1")
	u2 := insertAPITestUser(t, d, "mr-u2")
	muxAsU2 := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: u2, Username: "mr-u2"}})

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: u1, Title: "F", FeedURL: "https://x/feed", Created: time.Now().Unix(),
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	muxAsU2.ServeHTTP(w, httptest.NewRequest("POST",
		fmt.Sprintf("/api/v1/subscriptions/%d/mark-read", subID), nil))
	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSubscriptionsAPI_MarkRead_BadID(t *testing.T) {
	t.Parallel()
	d := newTestDB(t)
	userID := insertAPITestUser(t, d, "subuser-bad")
	mux := NewTestMux(d, TestMuxOpts{TestUser: db.User{ID: userID, Username: "subuser-bad"}})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/subscriptions/abc/mark-read", nil))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

// newPatchHarness spins up an in-memory DB with one user and one subscription, builds a
// mux with the supplied poke counter, and returns everything the caller needs.
func newPatchHarness(t *testing.T, poked *int) (db.Subscription, http.Handler, string, string, *sql.DB) {
	t.Helper()
	d := newTestDB(t)
	uid := insertAPITestUser(t, d, "patch-user")
	u := db.User{ID: uid, Username: "patch-user", Role: "admin"}
	sess := db.Session{ID: 1, UserID: uid, CSRFToken: "test-csrf"}

	subID, err := db.InsertSubscription(context.Background(), d, db.NewSubscription{
		UserID: uid, Title: "Test Feed", FeedURL: "https://patch.example/feed", NextPoll: 0, Created: 0,
	})
	require.NoError(t, err)
	sub, err := db.GetSubscription(context.Background(), d, subID, uid)
	require.NoError(t, err)

	mux := NewTestMux(d, TestMuxOpts{
		MuxOpts:     MuxOpts{Poke: func() { *poked++ }},
		TestUser:    u,
		TestSession: sess,
	})
	return sub, mux, "tap_session", "test-csrf", d
}

func TestPatchSubscriptionRefreshNow(t *testing.T) {
	t.Parallel()
	poked := 0
	sub, mux, sessionCookie, csrfToken, d := newPatchHarness(t, &poked)

	futureTime := time.Now().Add(time.Hour).Unix()
	_, err := d.ExecContext(t.Context(),
		`UPDATE subscriptions SET next_poll_at = ? WHERE id = ?`, futureTime, sub.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(sub.ID, 10),
		strings.NewReader(`{"refresh_now":true}`))
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: sessionCookie})
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var dto subscriptionDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dto))
	require.Equal(t, int64(0), dto.NextPollAt, "next_poll_at must reset to 0")
	require.Equal(t, 1, poked, "scheduler must be poked exactly once")
}

func TestPatchSubscriptionRefreshNow_WrongType(t *testing.T) {
	t.Parallel()
	poked := 0
	sub, mux, sessionCookie, csrfToken, _ := newPatchHarness(t, &poked)

	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(sub.ID, 10),
		strings.NewReader(`{"refresh_now":"yes"}`))
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: sessionCookie})
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Zero(t, poked)
}

func TestPatchSubscriptionRefreshNow_False(t *testing.T) {
	t.Parallel()
	poked := 0
	sub, mux, sessionCookie, csrfToken, d := newPatchHarness(t, &poked)

	futureTime := time.Now().Add(time.Hour).Unix()
	_, err := d.ExecContext(t.Context(),
		`UPDATE subscriptions SET next_poll_at = ? WHERE id = ?`, futureTime, sub.ID)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPatch,
		"/api/v1/subscriptions/"+strconv.FormatInt(sub.ID, 10),
		strings.NewReader(`{"refresh_now":false}`))
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: sessionCookie})
	req.Header.Set("X-CSRF-Token", csrfToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var dto subscriptionDTO
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &dto))
	require.Equal(t, futureTime, dto.NextPollAt, "next_poll_at must be unchanged")
	require.Zero(t, poked)
}
