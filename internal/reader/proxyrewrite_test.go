package reader_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bcrisp4/tap/internal/reader"
)

// stubEncode is the test-only proxy encoder: the URL is the original
// prefixed with /p/ so assertions can check both the prefix and that
// the source URL was resolved.
func stubEncode(u string) string { return "/p/" + u }

func TestRewriteMedia_RewritesImgSrc(t *testing.T) {
	in := `<p>x</p><img src="/img/x.png"/><p>y</p>`
	out, err := reader.RewriteMedia(in, "https://example.com/post/1", stubEncode)
	require.NoError(t, err)
	require.Contains(t, out, `src="/p/https://example.com/img/x.png"`)
	require.NotContains(t, out, `src="/img/x.png"`)
}

func TestRewriteMedia_RewritesSrcsetWithDescriptors(t *testing.T) {
	in := `<img src="/a.png" srcset="/a.png 1x, /b.png 2x, https://cdn.example.com/c.png 1024w">`
	out, err := reader.RewriteMedia(in, "https://example.com/", stubEncode)
	require.NoError(t, err)

	// Each candidate URL rewritten; descriptors preserved verbatim.
	require.Contains(t, out, "/p/https://example.com/a.png 1x")
	require.Contains(t, out, "/p/https://example.com/b.png 2x")
	require.Contains(t, out, "/p/https://cdn.example.com/c.png 1024w")
}

func TestRewriteMedia_RewritesPictureSourceSrcset(t *testing.T) {
	in := `<picture>
		<source srcset="/wide.png 1024w" type="image/png">
		<img src="/fallback.png" alt="">
	</picture>`
	out, err := reader.RewriteMedia(in, "https://example.com/", stubEncode)
	require.NoError(t, err)
	require.Contains(t, out, "/p/https://example.com/wide.png 1024w")
	require.Contains(t, out, "/p/https://example.com/fallback.png")
}

func TestRewriteMedia_LeavesIframeAlone(t *testing.T) {
	in := `<iframe src="https://www.youtube.com/embed/X"></iframe>`
	out, err := reader.RewriteMedia(in, "https://x", stubEncode)
	require.NoError(t, err)
	require.Equal(t, strings.Count(in, "youtube.com"), strings.Count(out, "youtube.com"))
	require.NotContains(t, out, "/p/")
}

func TestRewriteMedia_DataURISkipped(t *testing.T) {
	in := `<img src="data:image/png;base64,abc">`
	out, err := reader.RewriteMedia(in, "https://x", stubEncode)
	require.NoError(t, err)
	require.NotContains(t, out, "/p/")
}
