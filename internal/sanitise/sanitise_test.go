package sanitise

import (
	"strings"
	"testing"
)

func TestSanitise_StripsScript(t *testing.T) {
	t.Parallel()
	in := `<p>hi</p><script>alert('xss')</script>`
	got := DefaultPolicy().Sanitise(in)
	if strings.Contains(got, "<script>") || strings.Contains(got, "alert") {
		t.Errorf("script not stripped: got %q", got)
	}
	if !strings.Contains(got, "<p>hi</p>") {
		t.Errorf("legitimate <p> stripped: got %q", got)
	}
}

func TestSanitise_DropsDangerousSchemes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
	}{
		{"javascript in href", `<a href="javascript:alert(1)">click</a>`},
		{"javascript in img src", `<img src="javascript:alert(1)">`},
		{"data in img src", `<img src="data:image/png;base64,AAAA">`},
		{"vbscript in href", `<a href="vbscript:msgbox(1)">click</a>`},
	}
	p := DefaultPolicy()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			for _, scheme := range []string{"javascript:", "data:", "vbscript:"} {
				if strings.Contains(got, scheme) {
					t.Errorf("scheme %q survived in %q output: %q", scheme, tc.name, got)
				}
			}
		})
	}
}

func TestSanitise_IframeAllowlist(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()

	allowed := []struct {
		name string
		src  string
	}{
		{"youtube embed", "https://www.youtube.com/embed/dQw4w9WgXcQ"},
		{"youtube embed bare", "https://youtube.com/embed/dQw4w9WgXcQ"},
		{"youtube-nocookie", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"},
		{"vimeo player", "https://player.vimeo.com/video/76979871"},
		{"bandcamp", "https://bandcamp.com/EmbeddedPlayer/album=12345/"},
		{"dailymotion", "https://dailymotion.com/embed/video/x123"},
		{"twitch player", "https://player.twitch.tv/?channel=foo"},
		{"spotify open", "https://open.spotify.com/embed/track/abc"},
		{"soundcloud", "https://soundcloud.com/oembed?url=foo"},
		{"soundcloud w", "https://w.soundcloud.com/player/?url=foo"},
		{"bilibili", "https://player.bilibili.com/player.html?aid=1"},
		{"vk", "https://vk.com/video_ext.php?oid=1&id=2"},
		{"framatube", "https://framatube.org/videos/embed/abc"},
		{"embedly cdn", "https://cdn.embedly.com/widgets/media.html?src=foo"},
	}
	for _, tc := range allowed {
		t.Run("allowed/"+tc.name, func(t *testing.T) {
			t.Parallel()
			in := `<p>before</p><iframe src="` + tc.src + `"></iframe><p>after</p>`
			got := p.Sanitise(in)
			if !strings.Contains(got, "<iframe") {
				t.Errorf("allowed iframe stripped: in=%q out=%q", in, got)
			}
			// Check for both unescaped and HTML-escaped forms of the URL
			// (html.Render escapes &, <, >, " in attribute values)
			escaped := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(tc.src, "&", "&amp;"), "<", "&lt;"), ">", "&gt;")
			if !strings.Contains(got, tc.src) && !strings.Contains(got, escaped) {
				t.Errorf("allowed iframe src missing: in=%q out=%q", in, got)
			}
		})
	}

	denied := []struct {
		name string
		src  string
	}{
		{"unknown host", "https://evil.example/embed"},
		{"subdomain attack on youtube", "https://youtube.com.evil.example/embed/x"},
		{"prefix attack on youtube", "https://evil.example/youtube.com/embed/x"},
		{"non-www subdomain of youtube", "https://m.youtube.com/embed/x"},
	}
	for _, tc := range denied {
		t.Run("denied/"+tc.name, func(t *testing.T) {
			t.Parallel()
			in := `<p>before</p><iframe src="` + tc.src + `"></iframe><p>after</p>`
			got := p.Sanitise(in)
			if strings.Contains(got, "<iframe") {
				t.Errorf("disallowed iframe survived: in=%q out=%q", in, got)
			}
		})
	}
}
