package discover_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/discover"
	"github.com/bcrisp4/tap/internal/httpx"
)

func testClient() *http.Client {
	return httpx.NewClient(httpx.Opts{SSRF: httpx.SSRFPolicy{Disabled: true}})
}

func TestDiscover_DirectFeedURL_Atom(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"><title>Test Atom Feed</title><link rel="alternate" href="https://example.com"/></feed>`))
	}))
	defer srv.Close()

	results, err := discover.Discover(context.Background(), testClient(), srv.URL)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "Test Atom Feed", results[0].Title)
	require.Equal(t, srv.URL, results[0].FeedURL)
	require.Equal(t, "atom", results[0].Type)
}

func TestDiscover_DirectFeedURL_RSS(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Test RSS Feed</title><link>https://example.com</link></channel></rss>`))
	}))
	defer srv.Close()

	results, err := discover.Discover(context.Background(), testClient(), srv.URL)
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "rss", results[0].Type)
}

func TestDiscover_HTMLPage_AlternateLinks(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head>
			<link rel="alternate" type="application/rss+xml" title="RSS Feed" href="/feed.xml"/>
			<link rel="alternate" type="application/atom+xml" title="Atom Feed" href="/atom.xml"/>
		</head><body></body></html>`))
	}))
	defer srv.Close()

	results, err := discover.Discover(context.Background(), testClient(), srv.URL)
	require.NoError(t, err)
	require.Len(t, results, 2)
	types := map[string]bool{}
	for _, r := range results {
		types[r.Type] = true
	}
	require.True(t, types["rss"])
	require.True(t, types["atom"])
}

func TestDiscover_NoFeeds_ReturnsErrNoFeeds(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>No feeds here</title></head><body></body></html>`))
	}))
	defer srv.Close()

	_, err := discover.Discover(context.Background(), testClient(), srv.URL)
	require.ErrorIs(t, err, discover.ErrNoFeeds)
}

func TestDiscover_JSONFeedType(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head>
			<link rel="alternate" type="application/feed+json" title="JSON Feed" href="/feed.json"/>
			<link rel="alternate" type="application/json" title="JSON Feed 2" href="/feed2.json"/>
		</head></html>`))
	}))
	defer srv.Close()

	results, err := discover.Discover(context.Background(), testClient(), srv.URL)
	require.NoError(t, err)
	require.Len(t, results, 2)
	for _, r := range results {
		require.Equal(t, "json", r.Type)
	}
}

func TestDiscover_SSRFBlocked_PrivateIP(t *testing.T) {
	t.Parallel()
	// Use an SSRF-aware client (not the permissive test client).
	client := httpx.NewClient(httpx.Opts{})

	// 192.168.1.1 is a private IP — SSRF policy should block it.
	_, err := discover.Discover(context.Background(), client, "http://192.168.1.1/feed")
	require.Error(t, err, "SSRF-blocked URL must return error")
}

func TestDiscover_NoAuthHeaders(t *testing.T) {
	t.Parallel()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0"?><rss version="2.0"><channel><title>Feed</title><link>https://example.com</link></channel></rss>`))
	}))
	defer srv.Close()

	client := testClient()
	// Even if we add a transport that could add auth, Discover should not set it.
	_, err := discover.Discover(context.Background(), client, srv.URL)
	require.NoError(t, err)
	require.Empty(t, gotAuth, "Discover must not send Authorization header")
}

func TestDiscover_MalformedURL_Error(t *testing.T) {
	t.Parallel()
	client := testClient()
	_, err := discover.Discover(context.Background(), client, "not-a-url")
	require.True(t, err != nil && !errors.Is(err, discover.ErrNoFeeds),
		"malformed URL must return an error that is not ErrNoFeeds, got: %v", err)
}
