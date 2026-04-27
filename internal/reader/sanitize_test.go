package reader_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/reader"
)

func defaultOpts() reader.SanitizeOptions {
	return reader.SanitizeOptions{
		ArticleURL:      "https://example.com/post/1",
		IframeAllowlist: reader.DefaultIframeHosts(),
	}
}

func TestSanitize_StripsScript(t *testing.T) {
	in := `<p>hi</p><script>alert(1)</script>`
	out, err := reader.Sanitize(in, defaultOpts())
	require.NoError(t, err)
	require.Contains(t, out, "<p>hi</p>")
	require.NotContains(t, out, "<script")
	require.NotContains(t, out, "alert(1)")
}

func TestSanitize_StripsInlineHandler(t *testing.T) {
	in := `<a href="https://x" onclick="bad()">link</a>`
	out, err := reader.Sanitize(in, defaultOpts())
	require.NoError(t, err)
	require.NotContains(t, out, "onclick")
}

func TestSanitize_DropsForbiddenIframeHost(t *testing.T) {
	in := `<iframe src="https://evil.example.com/x"></iframe>` +
		`<iframe src="https://www.youtube.com/embed/abc"></iframe>`
	out, err := reader.Sanitize(in, defaultOpts())
	require.NoError(t, err)
	require.NotContains(t, out, "evil.example.com")
	require.Contains(t, out, "youtube.com")
}

func TestSanitize_StripsTrackingPixel(t *testing.T) {
	in := `<img src="https://t.example/px" width="1" height="1"><img src="https://x/y.png">`
	out, err := reader.Sanitize(in, defaultOpts())
	require.NoError(t, err)
	require.NotContains(t, out, "t.example")
	require.Contains(t, out, "y.png")
}

func TestSanitize_AllowsDataURIForImageMIME(t *testing.T) {
	in := `<img src="data:image/png;base64,iVBORw0K">`
	out, err := reader.Sanitize(in, defaultOpts())
	require.NoError(t, err)
	require.Contains(t, out, "data:image/png")
}

func TestSanitize_DropsDataURIForOtherMIME(t *testing.T) {
	in := `<img src="data:application/javascript;base64,YWxlcnQoMSk=">`
	out, err := reader.Sanitize(in, defaultOpts())
	require.NoError(t, err)
	require.NotContains(t, out, "data:application/javascript")
}

func TestSanitize_ResolvesRelativeHref(t *testing.T) {
	in := `<a href="/about">about</a>`
	out, err := reader.Sanitize(in, defaultOpts())
	require.NoError(t, err)
	require.Contains(t, out, `href="https://example.com/about"`)
}

func TestSanitize_AllowedFormattingPreserved(t *testing.T) {
	in := `<h2>Title</h2><p><strong>bold</strong> and <em>italic</em>.</p>` +
		`<pre><code>x := 1</code></pre><blockquote>q</blockquote>`
	out, err := reader.Sanitize(in, defaultOpts())
	require.NoError(t, err)
	for _, want := range []string{"<h2>", "<strong>", "<em>", "<pre>", "<code>", "<blockquote>"} {
		require.True(t, strings.Contains(out, want), "expected %q in %q", want, out)
	}
}
