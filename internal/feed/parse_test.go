package feed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const sampleAtom = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Sample</title>
  <link href="https://sample.example/"/>
  <id>urn:sample</id>
  <updated>2026-05-01T00:00:00Z</updated>
  <entry>
    <title>First post</title>
    <id>urn:sample:1</id>
    <link href="https://sample.example/1"/>
    <updated>2026-05-01T00:00:00Z</updated>
    <content type="html">&lt;p&gt;hello&lt;/p&gt;</content>
  </entry>
</feed>`

func TestFetch_OK(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sampleAtom))
	}))
	defer srv.Close()

	res, err := Fetch(context.Background(), http.DefaultClient, srv.URL, FetchOpts{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.Status)
	require.Equal(t, `"abc"`, res.ETag)
	require.NotNil(t, res.Feed)
	require.Len(t, res.Feed.Items, 1)
	require.Equal(t, "First post", res.Feed.Items[0].Title)
}

func TestFetch_NotModified(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, `"prev"`, r.Header.Get("If-None-Match"))
		require.Equal(t, "Wed, 01 May 2026 00:00:00 GMT", r.Header.Get("If-Modified-Since"))
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	res, err := Fetch(context.Background(), http.DefaultClient, srv.URL, FetchOpts{
		PriorETag:         `"prev"`,
		PriorLastModified: "Wed, 01 May 2026 00:00:00 GMT",
	})
	require.NoError(t, err)
	require.Equal(t, http.StatusNotModified, res.Status)
	require.Nil(t, res.Feed)
}

func TestFetch_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := Fetch(context.Background(), http.DefaultClient, srv.URL, FetchOpts{})
	require.Error(t, err)
}

func TestFetch_DoesNotSetUserAgent(t *testing.T) {
	t.Parallel()
	var sawUA atomic.Value
	sawUA.Store("")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawUA.Store(r.Header.Get("User-Agent"))
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	_, err := Fetch(context.Background(), srv.Client(), srv.URL, FetchOpts{})
	require.NoError(t, err)
	ua := sawUA.Load().(string)
	// The shared httpx client injects the tap UA. feed.Fetch itself must
	// not set "tap/" — that string is owned by httpx.Opts.UserAgent.
	require.NotContains(t, ua, "tap/", "Fetch should not set tap-specific UA")
}

func TestFetch_PopulatesRetryAfterOnError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "120")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	res, err := Fetch(context.Background(), srv.Client(), srv.URL, FetchOpts{})
	require.Error(t, err, "expected error on 503")
	require.False(t, res.RetryAfter.IsZero(), "RetryAfter should be populated even on error")
}

func TestFetch_PopulatesCacheMaxAgeOnSuccess(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(`<rss version="2.0"><channel><title>t</title></channel></rss>`))
	}))
	t.Cleanup(srv.Close)

	res, err := Fetch(context.Background(), srv.Client(), srv.URL, FetchOpts{})
	require.NoError(t, err)
	require.Equal(t, 3600*time.Second, res.CacheMaxAge)
}
