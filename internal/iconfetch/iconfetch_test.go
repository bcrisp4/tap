package iconfetch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/httpclient"
	"github.com/bcrisp4/tap/internal/iconfetch"
)

func newAllowingClient(t *testing.T) *httpclient.Client {
	t.Helper()
	return httpclient.NewClient(httpclient.Config{
		AllowPrivate: true, // 127.0.0.1 + httptest server
	})
}

func TestFromHTML_PicksFirstIconLink(t *testing.T) {
	body := []byte(`<html><head>
		<link rel="alternate" href="/feed.xml">
		<link rel="icon" href="/favicon.png">
		<link rel="shortcut icon" href="/legacy.ico">
		<link rel="apple-touch-icon" href="/apple.png">
	</head></html>`)

	got := iconfetch.FromHTML(body, "https://example.com/article")
	require.Equal(t, []string{
		"https://example.com/favicon.png",
		"https://example.com/legacy.ico",
		"https://example.com/apple.png",
	}, got)
}

func TestFromHTML_IgnoresNonIconRels(t *testing.T) {
	body := []byte(`<html><head>
		<link rel="alternate" href="/feed.xml">
		<link rel="stylesheet" href="/x.css">
	</head></html>`)
	require.Empty(t, iconfetch.FromHTML(body, "https://example.com/"))
}

// Bare `rel="shortcut"` (without an accompanying `icon` token) is
// historically used for unrelated resources and must not be treated
// as a favicon candidate. Real legacy markup always pairs them as
// `rel="shortcut icon"`.
func TestFromHTML_BareShortcutIsNotIcon(t *testing.T) {
	body := []byte(`<html><head>
		<link rel="shortcut" href="/not-a-favicon">
		<link rel="shortcut icon" href="/legacy.ico">
	</head></html>`)
	got := iconfetch.FromHTML(body, "https://example.com/")
	require.Equal(t, []string{"https://example.com/legacy.ico"}, got)
}

func TestFaviconFallback(t *testing.T) {
	require.Equal(t, "https://example.com/favicon.ico",
		iconfetch.FaviconFallback("https://example.com/some/path?x=1"))
	require.Equal(t, "", iconfetch.FaviconFallback("not a url"))
}

func TestFetcher_FetchPNG(t *testing.T) {
	pngHeader := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x01, 0x02}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngHeader)
	}))
	defer srv.Close()

	f := iconfetch.New(newAllowingClient(t))
	res, err := f.Fetch(context.Background(), srv.URL+"/favicon.png")
	require.NoError(t, err)
	require.Equal(t, "image/png", res.MIME)
	require.Equal(t, pngHeader, res.Bytes)
}

func TestFetcher_RejectsDisallowedMIME(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>not an icon</html>"))
	}))
	defer srv.Close()

	f := iconfetch.New(newAllowingClient(t))
	_, err := f.Fetch(context.Background(), srv.URL+"/no.html")
	require.Error(t, err)
}

func TestFetcher_FallsBackOnExtensionWhenContentTypeIsGeneric(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Some CDNs serve .ico as application/octet-stream.
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte{0x00, 0x00, 0x01, 0x00})
	}))
	defer srv.Close()

	f := iconfetch.New(newAllowingClient(t))
	res, err := f.Fetch(context.Background(), srv.URL+"/favicon.ico")
	require.NoError(t, err)
	require.Equal(t, "image/x-icon", res.MIME)
}

func TestFetcher_RejectsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	f := iconfetch.New(newAllowingClient(t))
	_, err := f.Fetch(context.Background(), srv.URL+"/missing.png")
	require.Error(t, err)
}

func TestDiscover_PrefersFirstWorking(t *testing.T) {
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		switch r.URL.Path {
		case "/missing.png":
			http.NotFound(w, r)
		case "/found.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(pngBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := iconfetch.New(newAllowingClient(t))
	res, err := f.Discover(context.Background(), []string{
		srv.URL + "/missing.png",
		srv.URL + "/found.png",
		srv.URL + "/favicon.ico",
	})
	require.NoError(t, err)
	require.Equal(t, "image/png", res.MIME)
	require.Equal(t, pngBytes, res.Bytes)
	// /favicon.ico must NOT have been hit — Discover returns on first
	// success.
	require.Equal(t, 0, calls["/favicon.ico"])
}

func TestDiscover_AllFailReturnsErrNoIcon(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.NotFound(w, &http.Request{}) //nolint:exhaustruct
		_ = w
	}))
	defer srv.Close()

	f := iconfetch.New(newAllowingClient(t))
	_, err := f.Discover(context.Background(), []string{
		srv.URL + "/a", srv.URL + "/b",
	})
	require.ErrorIs(t, err, iconfetch.ErrNoIcon)
}

func TestCandidatesFor_HTMLLinksThenFallback(t *testing.T) {
	body := []byte(`<html><head>
		<link rel="icon" href="/site.png">
	</head></html>`)
	got := iconfetch.CandidatesFor(body, "https://example.com/article")
	require.Equal(t, []string{
		"https://example.com/site.png",
		"https://example.com/favicon.ico",
	}, got)
}

func TestCandidatesFor_DropsNonHTTPSchemes(t *testing.T) {
	body := []byte(`<html><head>
		<link rel="icon" href="data:image/png;base64,abc">
		<link rel="shortcut icon" href="javascript:foo()">
		<link rel="icon" href="https://cdn.example.com/icon.png">
	</head></html>`)
	got := iconfetch.CandidatesFor(body, "https://example.com/")
	require.Equal(t, []string{
		"https://cdn.example.com/icon.png",
		"https://example.com/favicon.ico",
	}, got)
}

func TestCandidatesFor_DeDupesFallbackWhenAlreadyPresent(t *testing.T) {
	body := []byte(`<html><head>
		<link rel="icon" href="/favicon.ico">
	</head></html>`)
	got := iconfetch.CandidatesFor(body, "https://example.com/")
	require.Equal(t, []string{"https://example.com/favicon.ico"}, got)
}
