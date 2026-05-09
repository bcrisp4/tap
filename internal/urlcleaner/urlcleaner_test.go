package urlcleaner

import "testing"

func TestClean_StripsUtmSource(t *testing.T) {
	t.Parallel()
	got := Clean("https://example.com/page?utm_source=newsletter&id=42")
	want := "https://example.com/page?id=42"
	if got != want {
		t.Errorf("Clean() = %q, want %q", got, want)
	}
}

func TestClean_StripsKnownInboundTrackers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"utm_source", "https://e.com/?utm_source=x&keep=1", "https://e.com/?keep=1"},
		{"utm_medium", "https://e.com/?utm_medium=email&keep=1", "https://e.com/?keep=1"},
		{"utm_campaign", "https://e.com/?utm_campaign=fall&keep=1", "https://e.com/?keep=1"},
		{"utm_term", "https://e.com/?utm_term=foo&keep=1", "https://e.com/?keep=1"},
		{"utm_content", "https://e.com/?utm_content=bar&keep=1", "https://e.com/?keep=1"},
		{"mc_eid", "https://e.com/?mc_eid=abc&keep=1", "https://e.com/?keep=1"},
		{"mc_cid", "https://e.com/?mc_cid=def&keep=1", "https://e.com/?keep=1"},
		{"mkt_tok", "https://e.com/?mkt_tok=ghi&keep=1", "https://e.com/?keep=1"},
		{"hsCtaTracking", "https://e.com/?hsCtaTracking=jkl&keep=1", "https://e.com/?keep=1"},
		{"_hsmi", "https://e.com/?_hsmi=mno&keep=1", "https://e.com/?keep=1"},
		{"_hsenc", "https://e.com/?_hsenc=pqr&keep=1", "https://e.com/?keep=1"},
		{"vero_id", "https://e.com/?vero_id=stu&keep=1", "https://e.com/?keep=1"},
		{"vero_conv", "https://e.com/?vero_conv=abc&keep=1", "https://e.com/?keep=1"},
		{"oly_anon_id", "https://e.com/?oly_anon_id=vwx&keep=1", "https://e.com/?keep=1"},
		{"oly_enc_id", "https://e.com/?oly_enc_id=def&keep=1", "https://e.com/?keep=1"},
		{"wickedid", "https://e.com/?wickedid=yz&keep=1", "https://e.com/?keep=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Clean(tc.in); got != tc.want {
				t.Errorf("Clean(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestClean_StripsKnownOutboundTrackers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"fbclid", "https://e.com/?fbclid=a&keep=1", "https://e.com/?keep=1"},
		{"gclid", "https://e.com/?gclid=b&keep=1", "https://e.com/?keep=1"},
		{"dclid", "https://e.com/?dclid=c&keep=1", "https://e.com/?keep=1"},
		{"msclkid", "https://e.com/?msclkid=d&keep=1", "https://e.com/?keep=1"},
		{"yclid", "https://e.com/?yclid=e&keep=1", "https://e.com/?keep=1"},
		{"igshid", "https://e.com/?igshid=f&keep=1", "https://e.com/?keep=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Clean(tc.in); got != tc.want {
				t.Errorf("Clean(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestClean_StripsPrefixedTrackers(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"utm_unknown", "https://e.com/?utm_zzz=x&keep=1", "https://e.com/?keep=1"},
		{"mtm_source", "https://e.com/?mtm_source=x&keep=1", "https://e.com/?keep=1"},
		{"mtm_unknown", "https://e.com/?mtm_anything=x&keep=1", "https://e.com/?keep=1"},
		{"pk_campaign", "https://e.com/?pk_campaign=x&keep=1", "https://e.com/?keep=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Clean(tc.in); got != tc.want {
				t.Errorf("Clean(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestClean_LegitimateParamsSurvive(t *testing.T) {
	t.Parallel()
	in := "https://example.com/article?id=42&page=3&q=hello"
	if got := Clean(in); got != in {
		t.Errorf("Clean(%q) modified URL with no tracking params: got %q", in, got)
	}
}

func TestClean_UnchangedOnNoTrackingParams(t *testing.T) {
	t.Parallel()
	in := "https://example.com/article"
	if got := Clean(in); got != in {
		t.Errorf("Clean(%q) modified URL with no query string: got %q", in, got)
	}
}

func TestClean_NonURLReturnsUnchanged(t *testing.T) {
	t.Parallel()
	// Inputs urlcleaner cannot make sense of: empty string, junk, malformed
	// URL with no scheme. urlcleaner is not a security boundary — dangerous
	// schemes (javascript:, data:) are dropped by sanitise's URL allowlist
	// upstream, not here.
	for _, in := range []string{"", "not a url", "://malformed"} {
		if got := Clean(in); got != in {
			t.Errorf("Clean(%q) = %q, want unchanged", in, got)
		}
	}
}

func TestClean_NoTrailingQuestionMarkAfterStrip(t *testing.T) {
	t.Parallel()
	in := "https://example.com/article?utm_source=foo"
	want := "https://example.com/article"
	if got := Clean(in); got != want {
		t.Errorf("Clean(%q) = %q, want %q (no trailing '?')", in, got, want)
	}
}
