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

var _ = strings.TrimSpace // used in later tasks
