package processor_test

import (
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/processor"
	"github.com/bcrisp4/tap/internal/sanitise"
	"github.com/stretchr/testify/require"
)

func TestProcessor_NilRewriter_MatchesSanitiseBytewise(t *testing.T) {
	t.Parallel()
	pol := sanitise.DefaultPolicy()
	p := processor.New(pol, nil)

	cases := []string{
		`<p>hello</p>`,
		`<p>x</p><script>alert(1)</script>`,
		`<a href="https://example.com/?utm_source=x">link</a>`,
		`<img src="https://example.com/img.png">`,
		`<iframe src="https://www.youtube.com/embed/abc"></iframe>`,
		``,
	}
	for _, in := range cases {
		require.Equal(t, pol.Sanitise(in), p.Process(in), "input: %q", in)
	}
}

func TestProcessor_RewriterReplacesImgSrc(t *testing.T) {
	t.Parallel()
	pol := sanitise.DefaultPolicy()
	rewriter := func(s string) string { return "/PROXY/" + s }
	p := processor.New(pol, rewriter)

	in := `<img src="https://example.com/img.png">`
	out := p.Process(in)
	require.Contains(t, out, `src="/PROXY/https://example.com/img.png"`)
	require.NotContains(t, out, `src="https://example.com/img.png"`, "raw URL must be replaced: %s", out)
}

func TestProcessor_RewriterDoesNotTouchAnchorOrIframe(t *testing.T) {
	t.Parallel()
	pol := sanitise.DefaultPolicy()
	rewriter := func(s string) string { return "/PROXY/" + s }
	p := processor.New(pol, rewriter)

	in := `<a href="https://example.com/page">x</a><iframe src="https://www.youtube.com/embed/abc"></iframe>`
	out := p.Process(in)
	require.Contains(t, out, `href="https://example.com/page"`)
	require.Contains(t, out, `src="https://www.youtube.com/embed/abc"`)
	require.NotContains(t, out, "/PROXY/", "rewriter must touch <img> only: %s", out)
}

func TestNew_PanicsOnNilSanitiser(t *testing.T) {
	t.Parallel()
	require.PanicsWithValue(t, "processor.New: sanitiser is required", func() {
		processor.New(nil, nil)
	})
}

func TestProcessor_TotalFunctionContract(t *testing.T) {
	t.Parallel()
	pol := sanitise.DefaultPolicy()
	rewriter := func(s string) string { return "/PROXY/" + s }
	p := processor.New(pol, rewriter)

	bad := []string{
		"",
		"<<<<<",
		strings.Repeat("<img src='x'>", 1000),
		string([]byte{0xff, 0xfe, 0xfd}),
	}
	for _, in := range bad {
		// Must not panic; must return a string.
		_ = p.Process(in)
	}
}
