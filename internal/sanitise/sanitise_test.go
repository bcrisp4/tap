package sanitise

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"unicode/utf8"
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

func TestSanitise_PixelTracker(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()

	dropped := []struct {
		name string
		in   string
	}{
		{"1x1", `<p>a</p><img src="https://t.example/p" width="1" height="1"><p>b</p>`},
		{"0x0", `<p>a</p><img src="https://t.example/p" width="0" height="0"><p>b</p>`},
		{"1x0", `<p>a</p><img src="https://t.example/p" width="1" height="0"><p>b</p>`},
		{"0x1", `<p>a</p><img src="https://t.example/p" width="0" height="1"><p>b</p>`},
	}
	for _, tc := range dropped {
		t.Run("dropped/"+tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			if strings.Contains(got, "<img") {
				t.Errorf("pixel tracker survived: in=%q out=%q", tc.in, got)
			}
			if !strings.Contains(got, "<p>a</p>") || !strings.Contains(got, "<p>b</p>") {
				t.Errorf("surrounding content damaged: in=%q out=%q", tc.in, got)
			}
		})
	}

	kept := []struct {
		name string
		in   string
	}{
		{"1x2", `<img src="https://e.com/i" width="1" height="2">`},
		{"2x1", `<img src="https://e.com/i" width="2" height="1">`},
		{"5x5", `<img src="https://e.com/i" width="5" height="5">`},
		{"no dims", `<img src="https://e.com/i">`},
		{"only width", `<img src="https://e.com/i" width="1">`},
		{"only height", `<img src="https://e.com/i" height="1">`},
	}
	for _, tc := range kept {
		t.Run("kept/"+tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			if !strings.Contains(got, "<img") {
				t.Errorf("legitimate image dropped: in=%q out=%q", tc.in, got)
			}
		})
	}
}

func TestSanitise_URLCleanerIntegration(t *testing.T) {
	t.Parallel()
	p := DefaultPolicy()

	cases := []struct {
		name        string
		in          string
		mustContain string
		mustNot     []string
	}{
		{
			name:        "anchor utm_source",
			in:          `<a href="https://e.com/x?utm_source=foo&id=1">link</a>`,
			mustContain: `href="https://e.com/x?id=1"`,
			mustNot:     []string{"utm_source"},
		},
		{
			name:        "img fbclid",
			in:          `<img src="https://e.com/i.png?fbclid=bar&v=1">`,
			mustContain: `src="https://e.com/i.png?v=1"`,
			mustNot:     []string{"fbclid"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := p.Sanitise(tc.in)
			if !strings.Contains(got, tc.mustContain) {
				t.Errorf("missing expected fragment %q in output %q", tc.mustContain, got)
			}
			for _, ng := range tc.mustNot {
				if strings.Contains(got, ng) {
					t.Errorf("unwanted fragment %q in output %q", ng, got)
				}
			}
		})
	}
}

func TestSanitise_TruncatesOversizeInput(t *testing.T) {
	t.Parallel()
	// 2 MiB of "x" wrapped in a <p>; bluemonday should still produce
	// finite output and never see more than ~1 MiB of input.
	const wantCap = 1 << 20
	huge := "<p>" + strings.Repeat("x", 2*wantCap) + "</p>"
	got := DefaultPolicy().Sanitise(huge)
	// Slack of 256 covers any wrapping/balancing the HTML parser may
	// add when re-serialising the truncated fragment.
	if len(got) > wantCap+256 {
		t.Errorf("output size %d exceeds expected cap (%d + slack)", len(got), wantCap)
	}
}

func TestSanitise_TruncationSnapsToUTF8Boundary(t *testing.T) {
	t.Parallel()
	// Build input where byte position MaxInputBytes lands inside a
	// multi-byte UTF-8 codepoint. "€" is 3 bytes (E2 82 AC). Filling
	// up to MaxInputBytes-1 with ASCII then inserting "€" puts the
	// truncation cut mid-codepoint.
	const cap = 1 << 20
	body := strings.Repeat("a", cap-1) + "€" + strings.Repeat("b", 100)
	got := DefaultPolicy().Sanitise(body)
	// Result must be valid UTF-8 — no replacement chars from a partial
	// multi-byte sequence handed to the HTML parser.
	if !utf8.ValidString(got) {
		t.Errorf("output is not valid UTF-8")
	}
	if strings.Contains(got, "�") {
		t.Errorf("output contains U+FFFD replacement character (input was cut mid-codepoint)")
	}
}

func TestSanitise_LogsDropStats(t *testing.T) {
	// Capture slog output at DEBUG level via a buffer-backed handler.
	var buf bytes.Buffer
	h := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	in := `<p>ok</p>` +
		`<iframe src="https://evil.example/x"></iframe>` +
		`<img src="https://t.example/p" width="1" height="1">`
	_ = DefaultPolicy().Sanitise(in)

	out := buf.String()
	for _, want := range []string{`"sanitise"`, `"dropped_iframes":1`, `"dropped_pixel_trackers":1`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in slog output: %s", want, out)
		}
	}
}
