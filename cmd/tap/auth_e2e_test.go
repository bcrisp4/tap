package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/api"
	"github.com/bcrisp4/tap/internal/auth"
	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpx"
	"github.com/bcrisp4/tap/internal/poll"
	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/stretchr/testify/require"
)

// testServer bundles the moving parts of an end-to-end auth test: an
// httptest server fronting the real api.NewMux, plus the in-process pieces
// (DB, scheduler, signer) tests need to assert against.
//
// The harness mirrors what cmd/tap/main.go's runServer does, but runs
// in-process so tests don't have to launch the binary. Argon2 cost is
// pinned to testHashParams so the suite stays under a second.
type testServer struct {
	*httptest.Server
	d         *sql.DB
	signer    *proxy.Signer
	scheduler *poll.Scheduler
	dataDir   string
}

// shutdown stops the scheduler, closes the http server, and closes the DB.
// Used by tests that need to re-launch a server against the same data dir
// (verifying the bootstrap path is silent on a populated DB).
func (ts *testServer) shutdown() {
	ts.Server.Close()
	ts.scheduler.Stop()
	_ = ts.d.Close()
}

// startTestServer mirrors cmd/tap/main.go's runServer in-process:
//
//   - opens a fresh on-disk DB at <tmpdir>/tap.db (so the bootstrap path
//     and counts work as in production);
//   - applies migrations;
//   - runs the env-var bootstrap path with testHashParams (cheap argon2);
//   - builds the auth-wrapped api.NewMux mux;
//   - fronts the mux with httptest.NewServer (random port, http only).
//
// Returns a *testServer whose embedded *httptest.Server fronts the mux.
// Calling t.Cleanup is sufficient — the harness wires Close into Cleanup
// so callers don't have to re-do shutdown ordering.
//
// SSRF policy is permissive (loopback allowed) so any feed/origin servers
// the test stands up via httptest are reachable.
func startTestServer(t *testing.T) *testServer {
	t.Helper()
	return startTestServerInDir(t, t.TempDir())
}

// startTestServerInDir is like startTestServer but uses the supplied data
// directory rather than a fresh one — used by tests that need to re-launch
// the server against a DB populated by an earlier launch (e.g. the
// "silent on populated DB" bootstrap regression).
func startTestServerInDir(t *testing.T, dir string) *testServer {
	t.Helper()
	dbPath := filepath.Join(dir, "tap.db")
	ctx := context.Background()

	d, err := db.Open(ctx, dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(ctx, d))

	// Bootstrap path — same shape as runServer.
	if n, err := db.CountUsers(ctx, d); err != nil {
		t.Fatalf("count users: %v", err)
	} else if n == 0 {
		user := os.Getenv("TAP_ADMIN_USERNAME")
		pass := os.Getenv("TAP_ADMIN_PASSWORD")
		switch {
		case user != "" && pass != "":
			require.NoError(t, bootstrapAdmin(ctx, d, user, pass, testHashParams, io.Discard))
		case user != "" || pass != "":
			t.Fatal("partial admin bootstrap: both TAP_ADMIN_USERNAME and TAP_ADMIN_PASSWORD must be set")
		}
	}

	// Signer: persist a fresh key (same shape as runServer's
	// loadOrCreateProxyKey).
	key := make([]byte, proxy.KeySize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("generate signing key: %v", err)
	}
	gotKey, err := db.SetConfigIfAbsent(ctx, d, proxySigningKeyConfigKey, key)
	require.NoError(t, err)
	signer := proxy.NewSigner(gotKey)

	// Permissive SSRF policy: tests stand up httptest origins on 127.0.0.1.
	policy, err := httpx.ParseSSRFPolicy(false, []string{"127.0.0.0/8"})
	require.NoError(t, err)
	client := httpx.NewClient(httpx.Opts{
		Timeout:         5 * time.Second,
		PerHostInflight: 4,
		SSRF:            policy,
		UserAgent:       "tap-test/0.1",
	})

	cacheDir := filepath.Join(dir, "cache")
	require.NoError(t, os.MkdirAll(cacheDir, 0o755))
	cache := proxy.NewCache(cacheDir, 1<<20)
	proxyHandler := proxy.NewHandler(signer, cache, client, 10<<20)

	proc := processor.New(sanitise.DefaultPolicy(), signer.RewriteImageURL)
	sched := poll.NewScheduler(ctx, d, client, poll.SchedulerOpts{
		Workers:   1,
		Processor: proc,
		Floor:     15 * time.Minute,
		Ceiling:   24 * time.Hour,
	})
	sched.Start()
	t.Cleanup(sched.Stop)

	apiMux := api.NewMux(d, api.MuxOpts{
		Poke:               sched.Poke,
		ProxyHandler:       proxyHandler,
		SessionIdleTTL:     7 * 24 * time.Hour,
		SessionAbsoluteTTL: 90 * 24 * time.Hour,
		CookieSecure:       false, // httptest.Server is http://, can't set Secure
		HashParams:         testHashParams,
	})

	srv := httptest.NewServer(apiMux)
	t.Cleanup(srv.Close)

	return &testServer{
		Server:    srv,
		d:         d,
		signer:    signer,
		scheduler: sched,
		dataDir:   dir,
	}
}

// postJSON POSTs body (a raw JSON string) to path on srv. cookieValue, if
// non-empty, is sent as the tap_session cookie. Caller closes the response.
func postJSON(t *testing.T, srv *testServer, path, body, cookieValue string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL+path, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: "tap_session", Value: cookieValue})
	}
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	return resp
}

// sessionCookie reads the tap_session cookie from a response's Set-Cookie
// header. Fails the test if no such cookie was set.
func sessionCookie(t *testing.T, resp *http.Response) string {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == "tap_session" {
			return c.Value
		}
	}
	t.Fatalf("no tap_session cookie in response (cookies: %v)", resp.Cookies())
	return ""
}

// authedClient logs in as user with the given password and returns a small
// helper carrying the cookie + CSRF token for subsequent state-changing
// requests. Fails the test if login does not return 200.
type authClient struct {
	srv       *testServer
	t         *testing.T
	cookie    string
	csrfToken string
}

func authedClient(t *testing.T, srv *testServer, user, pass string) *authClient {
	t.Helper()
	loginBody, err := json.Marshal(map[string]string{"username": user, "password": pass})
	require.NoError(t, err)
	resp := postJSON(t, srv, "/api/v1/sessions", string(loginBody), "")
	defer resp.Body.Close()
	require.Equalf(t, http.StatusOK, resp.StatusCode, "login failed for user %q", user)

	cookie := sessionCookie(t, resp)
	var login struct {
		CSRFToken string `json:"csrf_token"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&login))
	require.NotEmpty(t, login.CSRFToken)

	return &authClient{srv: srv, t: t, cookie: cookie, csrfToken: login.CSRFToken}
}

// authResp is the simplified shape the helpers return.
type authResp struct {
	StatusCode int
	Body       string
	Header     http.Header
}

// Post sends a state-changing JSON request with the cookie + CSRF header.
func (c *authClient) Post(path, body string) authResp {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodPost, c.srv.URL+path, strings.NewReader(body))
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", c.csrfToken)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: c.cookie})
	resp, err := c.srv.Client().Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	return authResp{StatusCode: resp.StatusCode, Body: string(bs), Header: resp.Header.Clone()}
}

// Get sends an authenticated GET (no CSRF; safe methods skip the check).
func (c *authClient) Get(path string) authResp {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.srv.URL+path, nil)
	require.NoError(c.t, err)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: c.cookie})
	resp, err := c.srv.Client().Do(req)
	require.NoError(c.t, err)
	defer resp.Body.Close()
	bs, err := io.ReadAll(resp.Body)
	require.NoError(c.t, err)
	return authResp{StatusCode: resp.StatusCode, Body: string(bs), Header: resp.Header.Clone()}
}

// signProxyToken returns a signed proxy URL token for the given origin URL.
func signProxyToken(t *testing.T, srv *testServer, originURL string) string {
	t.Helper()
	return srv.signer.Sign(originURL)
}

// TestEndToEnd_LoginAndCSRFRoundtrip exercises the full middleware chain:
// log in, then POST /api/v1/subscriptions WITHOUT X-CSRF-Token (=> 403),
// then POST WITH X-CSRF-Token (=> 201). Pins the requireSession +
// requireCSRF composition end-to-end.
func TestEndToEnd_LoginAndCSRFRoundtrip(t *testing.T) {
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	srv := startTestServer(t)

	// Login.
	loginResp := postJSON(t, srv, "/api/v1/sessions",
		`{"username":"ben","password":"supersecret"}`, "")
	require.Equal(t, http.StatusOK, loginResp.StatusCode)
	defer loginResp.Body.Close()

	var login struct {
		User      struct{ Username, Role string } `json:"user"`
		CSRFToken string                          `json:"csrf_token"`
	}
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&login))
	require.Equal(t, "ben", login.User.Username)
	require.Equal(t, "admin", login.User.Role)
	require.NotEmpty(t, login.CSRFToken)

	cookie := sessionCookie(t, loginResp)

	// State-changing without CSRF => 403.
	rr := postJSON(t, srv, "/api/v1/subscriptions",
		`{"feed_url":"https://x.example/feed"}`, cookie)
	body, _ := io.ReadAll(rr.Body)
	rr.Body.Close()
	require.Equal(t, http.StatusForbidden, rr.StatusCode, "body=%s", string(body))

	// State-changing WITH CSRF => 201.
	body2, _ := json.Marshal(map[string]string{"feed_url": "https://x.example/feed"})
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/subscriptions", bytes.NewReader(body2))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", login.CSRFToken)
	req.AddCookie(&http.Cookie{Name: "tap_session", Value: cookie})
	resp, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

// TestEndToEnd_BasicAuthFlowsToFeedFetch verifies that basic_auth_*
// credentials persisted on a subscription reach the feed origin on the
// scheduler-driven poll. Pins the F-phase wiring end-to-end.
func TestEndToEnd_BasicAuthFlowsToFeedFetch(t *testing.T) {
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	gotAuth := make(chan string, 4)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case gotAuth <- r.Header.Get("Authorization"):
		default:
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>x</title><id>urn:x</id><updated>2026-05-01T00:00:00Z</updated></feed>`))
	}))
	t.Cleanup(origin.Close)

	srv := startTestServer(t)
	c := authedClient(t, srv, "ben", "supersecret")

	// Subscribe with basic auth set.
	subBody := fmt.Sprintf(`{"feed_url":%q,"basic_auth_user":"u","basic_auth_pass":"p"}`, origin.URL)
	resp := c.Post("/api/v1/subscriptions", subBody)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%s", resp.Body)
	require.Contains(t, resp.Body, `"has_basic_auth":true`)

	// startTestServer's scheduler runs sched.Poke on POST /subscriptions
	// (wired via MuxOpts.Poke). Wait for the origin to receive the request.
	select {
	case got := <-gotAuth:
		require.NotEmpty(t, got, "feed origin should have received Authorization: Basic ...")
		require.True(t, strings.HasPrefix(got, "Basic "), "expected Basic auth header, got %q", got)
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for poll-induced fetch")
	}
}

// TestEndToEnd_ProxyOriginIsAnonymous verifies that the media proxy's
// outbound origin fetch carries NO Cookie and NO Authorization header,
// even when the SPA's request to /api/v1/proxy/... was authenticated.
// This pins concept §6.7's anonymous-origin posture (Miniflux mirror).
func TestEndToEnd_ProxyOriginIsAnonymous(t *testing.T) {
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	type observed struct {
		auth   string
		cookie string
	}
	obs := make(chan observed, 4)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case obs <- observed{auth: r.Header.Get("Authorization"), cookie: r.Header.Get("Cookie")}:
		default:
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n" +
			"\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01\x08\x06\x00\x00\x00\x1f\x15\xc4\x89" +
			"\x00\x00\x00\x00IEND\xaeB`\x82"))
	}))
	t.Cleanup(origin.Close)

	srv := startTestServer(t)
	c := authedClient(t, srv, "ben", "supersecret")

	// Build a proxy URL via the harness's signer and GET it through the
	// authenticated client. The proxy handler must fetch the origin
	// anonymously, regardless of the inbound request's Cookie/Authorization.
	tok := signProxyToken(t, srv, origin.URL)
	resp := c.Get("/api/v1/proxy/" + tok)
	require.Equal(t, http.StatusOK, resp.StatusCode, "body=%s", resp.Body)

	select {
	case o := <-obs:
		require.Empty(t, o.auth, "proxy origin must not receive Authorization")
		require.Empty(t, o.cookie, "proxy origin must not receive Cookie")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for proxy origin fetch")
	}
}

// TestEndToEnd_LoginRequiresKnownCredentials makes sure that login with a
// non-existent user / wrong password returns 401 invalid_credentials and
// no session cookie.
func TestEndToEnd_LoginRequiresKnownCredentials(t *testing.T) {
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	srv := startTestServer(t)

	resp := postJSON(t, srv, "/api/v1/sessions",
		`{"username":"ben","password":"WRONG"}`, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	for _, c := range resp.Cookies() {
		require.NotEqualf(t, "tap_session", c.Name, "no session cookie on login failure")
	}
}

// TestEndToEnd_BootstrapFromEnvVarsCreatesAdmin pins the happy path of the
// env-var first-launch shortcut: empty DB + TAP_ADMIN_USERNAME +
// TAP_ADMIN_PASSWORD => admin row created, login works.
func TestEndToEnd_BootstrapFromEnvVarsCreatesAdmin(t *testing.T) {
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	srv := startTestServer(t)

	resp := postJSON(t, srv, "/api/v1/sessions",
		`{"username":"ben","password":"supersecret"}`, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Confirm exactly one user landed.
	n, err := db.CountUsers(context.Background(), srv.d)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

// TestEndToEnd_NoUsersNoEnvVarsRejectsLogin pins concept §7.1: empty DB
// without any env-var bootstrap means there is no SPA-visible bootstrap
// path. The server starts cleanly; login rejects everything.
func TestEndToEnd_NoUsersNoEnvVarsRejectsLogin(t *testing.T) {
	t.Setenv("TAP_ADMIN_USERNAME", "")
	t.Setenv("TAP_ADMIN_PASSWORD", "")

	srv := startTestServer(t)

	resp := postJSON(t, srv, "/api/v1/sessions",
		`{"username":"anyone","password":"anything"}`, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	n, err := db.CountUsers(context.Background(), srv.d)
	require.NoError(t, err)
	require.Equal(t, 0, n, "no user should be created")
}

// TestEndToEnd_BootstrapSilentOnPopulatedDB pins the "containers can keep
// the env vars set" contract: when users already exist, the bootstrap path
// is a silent no-op even if TAP_ADMIN_USERNAME / TAP_ADMIN_PASSWORD are
// still in the environment.
func TestEndToEnd_BootstrapSilentOnPopulatedDB(t *testing.T) {
	t.Setenv("TAP_ADMIN_USERNAME", "ben")
	t.Setenv("TAP_ADMIN_PASSWORD", "supersecret")

	dir := t.TempDir()

	// First launch bootstraps "ben". Tear it down so its DB handle is
	// released before the second launch reopens the same on-disk file.
	srv1 := startTestServerInDir(t, dir)
	srv1.shutdown()

	// Hand-add another user via the DB directly. This simulates `tap admin
	// create` having added a non-admin between launches.
	d, err := db.Open(context.Background(), filepath.Join(dir, "tap.db"))
	require.NoError(t, err)
	hash, err := auth.Hash("anotherpw", testHashParams)
	require.NoError(t, err)
	_, err = db.InsertUser(context.Background(), d, db.NewUser{
		Username: "alice", PasswordHash: hash, Role: "user", CreatedAt: 0,
	})
	require.NoError(t, err)
	require.NoError(t, d.Close())

	// Re-launch with env vars STILL set. Expect: silent no-op (no error,
	// no new admin row). User count must stay at 2.
	srv2 := startTestServerInDir(t, dir)

	n, err := db.CountUsers(context.Background(), srv2.d)
	require.NoError(t, err)
	require.Equal(t, 2, n, "no extra users should be created on the second launch")

	// Sanity: "ben" still works as the env-bootstrapped admin from the
	// first launch.
	resp := postJSON(t, srv2, "/api/v1/sessions",
		`{"username":"ben","password":"supersecret"}`, "")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}


