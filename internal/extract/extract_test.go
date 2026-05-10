package extract

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const articleHTML = `<!doctype html><html><head><title>Hello</title></head>
<body>
<header><nav><a href="/">home</a></nav></header>
<main>
<article>
<h1>The article title</h1>
<p>This is the first paragraph of a moderately long article body.
It needs enough text for Readability's heuristics to identify it as
the dominant content tree, otherwise the extractor returns empty.</p>
<p>This is the second paragraph, with similarly substantial prose so
the scorer picks &lt;article&gt; as the top candidate. Lorem ipsum dolor
sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt.</p>
<p>And a third paragraph just to be safe.</p>
</article>
</main>
<footer>copyright 2026</footer>
</body></html>`

func TestExtract_ReadabilityHappyPath(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(articleHTML))
	}))
	t.Cleanup(srv.Close)

	got, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.NoError(t, err)
	require.Contains(t, got, "first paragraph", "article body must survive extraction")
	require.NotContains(t, got, "copyright 2026", "footer must be dropped by Readability")
	require.NotContains(t, got, `href="/"`, "nav links must be dropped by Readability")
}

func TestExtract_HTTPNon2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "500")
}

func TestExtract_BodyCapExceeded(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("a", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(big))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 256)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cap")
}

func TestExtract_NonHTMLContentType(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"k":"v"}`))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.Error(t, err)
	require.Contains(t, err.Error(), "non-HTML")
}

func TestExtract_AcceptsXHTML(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xhtml+xml")
		_, _ = w.Write([]byte(articleHTML))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.NoError(t, err)
}

func TestExtract_MissingContentType(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// httptest auto-sets Content-Type to text/plain when body is non-empty
		// and no header is set; explicitly clear it to test the empty case.
		w.Header()["Content-Type"] = nil
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.Error(t, err)
}

func TestExtract_ReadabilityEmptyContent(t *testing.T) {
	t.Parallel()
	// A completely empty body causes Readability to return node==nil or
	// render an empty string; both paths return an error from Extract.
	const empty = `<!doctype html><html><head><title>Empty</title></head><body></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(empty))
	}))
	t.Cleanup(srv.Close)

	_, err := Extract(context.Background(), srv.Client(), srv.URL, "", 5<<20)
	require.Error(t, err, "Readability should error on a page with empty body")
}
