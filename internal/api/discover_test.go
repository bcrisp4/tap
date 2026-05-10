package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/db"
	"github.com/bcrisp4/tap/internal/httpx"
)

func TestDiscoverAPI_Candidates(t *testing.T) {
	t.Parallel()
	// Serve a mock feed page
	feedSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head>
			<link rel="alternate" type="application/rss+xml" title="My Feed" href="/feed.xml"/>
		</head></html>`))
	}))
	defer feedSrv.Close()

	sqlD := newTestAPIDB(t)
	userID := insertAPITestUser(t, sqlD, "disc1")
	client := httpx.NewClient(httpx.Opts{SSRF: httpx.SSRFPolicy{Disabled: true}})
	mux := NewTestMux(sqlD, TestMuxOpts{
		TestUser:       db.User{ID: userID, Username: "disc1", Role: "admin"},
		MuxOpts:        MuxOpts{DiscoverClient: client},
	})

	body := `{"url":"` + feedSrv.URL + `"}`
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/discover", strings.NewReader(body)))
	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Candidates []discoverCandidateDTO `json:"candidates"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Candidates, 1)
	require.Equal(t, "rss", resp.Candidates[0].Type)
}

func TestDiscoverAPI_NoFeeds(t *testing.T) {
	t.Parallel()
	emptySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head></head></html>`))
	}))
	defer emptySrv.Close()

	sqlD := newTestAPIDB(t)
	userID := insertAPITestUser(t, sqlD, "disc2")
	client := httpx.NewClient(httpx.Opts{SSRF: httpx.SSRFPolicy{Disabled: true}})
	mux := NewTestMux(sqlD, TestMuxOpts{
		TestUser: db.User{ID: userID, Username: "disc2", Role: "admin"},
		MuxOpts:  MuxOpts{DiscoverClient: client},
	})

	body := `{"url":"` + emptySrv.URL + `"}`
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/discover", strings.NewReader(body)))
	require.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Contains(t, string(resp["error"]), ErrCodeNoFeedsFound)
}

func TestDiscoverAPI_MalformedURL(t *testing.T) {
	t.Parallel()
	sqlD := newTestAPIDB(t)
	userID := insertAPITestUser(t, sqlD, "disc3")
	client := httpx.NewClient(httpx.Opts{SSRF: httpx.SSRFPolicy{Disabled: true}})
	mux := NewTestMux(sqlD, TestMuxOpts{
		TestUser: db.User{ID: userID, Username: "disc3", Role: "admin"},
		MuxOpts:  MuxOpts{DiscoverClient: client},
	})

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/discover", strings.NewReader(`{"url":"not-a-url"}`)))
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDiscoverAPI_Unauthenticated(t *testing.T) {
	t.Parallel()
	sqlD := newTestAPIDB(t)
	client := httpx.NewClient(httpx.Opts{SSRF: httpx.SSRFPolicy{Disabled: true}})
	mux := NewTestMux(sqlD, TestMuxOpts{MuxOpts: MuxOpts{DiscoverClient: client}}) // zero user

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/v1/discover", strings.NewReader(`{"url":"https://example.com"}`)))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}
