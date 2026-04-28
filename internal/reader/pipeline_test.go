package reader_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/reader"
)

func TestPipeline_ProcessHappyPath(t *testing.T) {
	html := `<article>
		<h2>Title</h2>
		<p>Body paragraph one with <a href="/foo">a link</a>.</p>
		<img src="/img/x.png">
		<script>alert(1)</script>
	</article>`

	p := reader.NewPipeline(reader.PipelineConfig{
		Encode:          stubEncode,
		IframeAllowlist: reader.DefaultIframeHosts(),
	})

	out, err := p.Process(html, "https://example.com/post/1", "")
	require.NoError(t, err)

	// Extracted (Readability or pass-through), media rewritten, sanitized.
	require.Contains(t, out, "Body paragraph one")
	require.Contains(t, out, `href="https://example.com/foo"`,
		"sanitizer must resolve relative href")
	require.Contains(t, out, "/p/https://example.com/img/x.png",
		"media rewriter must run before sanitize")
	require.NotContains(t, out, "<script")
	require.NotContains(t, out, "alert(1)")
}

func TestPipeline_RuleBeatsReadability(t *testing.T) {
	html := `<aside>SIDEBAR</aside><main><article class="post">BODY</article></main>`
	p := reader.NewPipeline(reader.PipelineConfig{Encode: stubEncode, IframeAllowlist: reader.DefaultIframeHosts()})
	out, err := p.Process(html, "https://x/", "main article.post")
	require.NoError(t, err)
	require.Contains(t, out, "BODY")
	require.NotContains(t, strings.ToLower(out), "sidebar")
}

// TestPipeline_RewriteAndSanitize_Standalone verifies that the universal
// middle+last steps are reachable independently of Extract. Plan 15
// splits the pipeline so non-crawler feeds — whose content is already a
// body fragment — can flow through the proxy + sanitiser without going
// through Readability first.
func TestPipeline_RewriteAndSanitize_Standalone(t *testing.T) {
	// A typical feed-supplied summary fragment: a paragraph plus an
	// image with a relative src, plus a script the sanitizer must drop.
	html := `<p>Body para with <a href="/foo">a link</a>.</p>` +
		`<img src="/img/x.png">` +
		`<script>alert(1)</script>`

	p := reader.NewPipeline(reader.PipelineConfig{
		Encode:          stubEncode,
		IframeAllowlist: reader.DefaultIframeHosts(),
	})

	out, err := p.RewriteAndSanitize(html, "https://example.com/post/1")
	require.NoError(t, err)

	require.Contains(t, out, "Body para")
	require.Contains(t, out, `href="https://example.com/foo"`,
		"sanitizer must resolve relative href")
	require.Contains(t, out, "/p/https://example.com/img/x.png",
		"media rewriter must run before sanitize")
	require.NotContains(t, out, "<script")
	require.NotContains(t, out, "alert(1)")
}

// TestPipeline_RewriteAndSanitize_AlwaysSanitizes guards the security
// invariant baked into the pipeline split: any path that returns a
// non-error value from RewriteAndSanitize MUST have run through the
// sanitizer, even if RewriteMedia internally errored. The frontend
// renders content with `{@html entry.content}` so a path that bypasses
// sanitisation on the rewrite-error fallback would be a stored-XSS
// vector. We can't easily force RewriteMedia to fail (goquery is
// lenient) so we exercise the success path and assert the script tag
// is dropped — the regression we'd worry about is RewriteAndSanitize
// returning unsanitised output on its degraded fallback path.
func TestPipeline_RewriteAndSanitize_AlwaysSanitizes(t *testing.T) {
	html := `<p>x</p><script>alert(1)</script><img src="/img/y.png">`
	p := reader.NewPipeline(reader.PipelineConfig{
		Encode:          stubEncode,
		IframeAllowlist: reader.DefaultIframeHosts(),
	})
	out, err := p.RewriteAndSanitize(html, "https://example.com/post")
	require.NoError(t, err)
	require.NotContains(t, out, "<script")
	require.NotContains(t, out, "alert(1)")
	require.Contains(t, out, "/p/https://example.com/img/y.png")
}

func TestPipeline_Extract_Standalone(t *testing.T) {
	// Extract is exposed as a method so callers can compose the pipeline
	// piecewise (crawler-only extraction, then universal sanitize).
	html := `<aside>SIDEBAR</aside><main><article class="post">BODY</article></main>`
	p := reader.NewPipeline(reader.PipelineConfig{Encode: stubEncode, IframeAllowlist: reader.DefaultIframeHosts()})

	out, err := p.Extract(html, "https://x/", "main article.post")
	require.NoError(t, err)
	require.Contains(t, out, "BODY")
	require.NotContains(t, strings.ToLower(out), "sidebar")
}
