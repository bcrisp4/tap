package httpx

import (
	"errors"
	"net/http"
	"net/netip"
	"net/url"
	"testing"
)

func TestAllowAddr_DefaultRejects(t *testing.T) {
	cases := []string{
		"127.0.0.1",
		"10.0.0.1",
		"172.16.5.5",
		"192.168.1.1",
		"169.254.1.1",
		"100.64.1.1",
		"0.0.0.0",
		"::1",
		"fe80::1",
		"fc00::1",
		"::",
		"::ffff:127.0.0.1",
	}
	p := SSRFPolicy{}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			addr := netip.MustParseAddr(s)
			err := p.AllowAddr(addr)
			if err == nil {
				t.Errorf("expected reject for %s, got nil", s)
			}
			if !errors.Is(err, ErrSSRFBlocked) {
				t.Errorf("expected ErrSSRFBlocked, got %v", err)
			}
		})
	}
}

func TestAllowAddr_PublicAccepted(t *testing.T) {
	p := SSRFPolicy{}
	cases := []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			addr := netip.MustParseAddr(s)
			if err := p.AllowAddr(addr); err != nil {
				t.Errorf("expected allow for %s, got %v", s, err)
			}
		})
	}
}

func TestAllowAddr_Disabled(t *testing.T) {
	p := SSRFPolicy{Disabled: true}
	addr := netip.MustParseAddr("127.0.0.1")
	if err := p.AllowAddr(addr); err != nil {
		t.Errorf("Disabled=true should allow loopback, got %v", err)
	}
}

func TestAllowAddr_CIDRAllowlist(t *testing.T) {
	p := SSRFPolicy{
		AllowCIDRs: []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")},
	}
	if err := p.AllowAddr(netip.MustParseAddr("192.168.1.5")); err != nil {
		t.Errorf("192.168.1.5 should be allowlisted, got %v", err)
	}
	if err := p.AllowAddr(netip.MustParseAddr("192.168.2.5")); err == nil {
		t.Errorf("192.168.2.5 should be rejected (outside allowlist)")
	}
}

// TestAllowAddr_V4MappedAgainstV4Allowlist guards against asymmetry between
// the allowlist and reject loops in AllowAddr. The reject loop unmaps
// v4-mapped-v6 inputs so an attacker can't bypass with ::ffff:127.0.0.1;
// the allowlist must do the same so a legitimate v4 CIDR entry covers the
// same v4-mapped form. Otherwise spec line 84 ("matches either kind of
// allowlist entry skips the reject rules entirely") is violated.
func TestAllowAddr_V4MappedAgainstV4Allowlist(t *testing.T) {
	p := SSRFPolicy{
		AllowCIDRs: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")},
	}
	if err := p.AllowAddr(netip.MustParseAddr("::ffff:127.0.0.1")); err != nil {
		t.Errorf("v4-mapped 127.0.0.1 should be allowlisted by 127.0.0.0/8, got %v", err)
	}
}

func TestAllowHostname_Suffix(t *testing.T) {
	p := SSRFPolicy{AllowSuffixes: []string{"home.lan", "ts.net"}}
	cases := []struct {
		host string
		want bool
	}{
		{"home.lan", true},
		{"nas.home.lan", true},
		{"deeply.nested.home.lan", true},
		{"notmyhome.lan", false},
		{"home.lan.evil.com", false},
		{"NAS.HOME.LAN", true},
		{"foo.ts.net", true},
		{"barts.net", false},
		{"unrelated.com", false},
		// Absolute DNS form with trailing dot — defence-in-depth: a feed URL
		// like http://nas.home.lan./feed.xml must still match.
		{"nas.home.lan.", true},
		{"home.lan.", true},
	}
	for _, tc := range cases {
		t.Run(tc.host, func(t *testing.T) {
			if got := p.AllowHostname(tc.host); got != tc.want {
				t.Errorf("AllowHostname(%q) = %v; want %v", tc.host, got, tc.want)
			}
		})
	}
}

func TestAllowHostname_DisabledAllowsAll(t *testing.T) {
	p := SSRFPolicy{Disabled: true}
	if !p.AllowHostname("anything.example") {
		t.Error("Disabled should allow any hostname")
	}
}

func TestParseSSRFPolicy_Empty(t *testing.T) {
	p, err := ParseSSRFPolicy(false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Disabled || len(p.AllowSuffixes) != 0 || len(p.AllowCIDRs) != 0 {
		t.Errorf("expected empty policy, got %+v", p)
	}
}

func TestParseSSRFPolicy_AutoDetect(t *testing.T) {
	p, err := ParseSSRFPolicy(false, []string{
		"192.168.1.0/24",
		"127.0.0.1",
		"::1",
		"marlin-tet.ts.net",
		"  10.0.0.0/8  ",
		"",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.AllowCIDRs) != 4 {
		t.Errorf("AllowCIDRs len = %d; want 4 (got %v)", len(p.AllowCIDRs), p.AllowCIDRs)
	}
	if len(p.AllowSuffixes) != 1 || p.AllowSuffixes[0] != "marlin-tet.ts.net" {
		t.Errorf("AllowSuffixes = %v", p.AllowSuffixes)
	}
}

func TestParseSSRFPolicy_MalformedCIDR(t *testing.T) {
	_, err := ParseSSRFPolicy(false, []string{"not/a/cidr"})
	if err == nil {
		t.Error("expected error for malformed entry")
	}
}

func TestParseSSRFPolicy_NormalisesTrailingDot(t *testing.T) {
	// Operator entering "home.lan." (DNS-absolute form) should match
	// "nas.home.lan" and "home.lan" — without the suffix-side normalisation
	// the trailing dot would defeat the dot-boundary match.
	p, err := ParseSSRFPolicy(false, []string{"home.lan.", "TS.NET."})
	if err != nil {
		t.Fatal(err)
	}
	if !p.AllowHostname("home.lan") {
		t.Error("home.lan should match suffix entered as 'home.lan.'")
	}
	if !p.AllowHostname("nas.home.lan") {
		t.Error("nas.home.lan should match suffix entered as 'home.lan.'")
	}
	if !p.AllowHostname("foo.ts.net") {
		t.Error("foo.ts.net should match suffix entered as 'TS.NET.'")
	}
}

func TestParseSSRFPolicy_DisabledFlag(t *testing.T) {
	p, err := ParseSSRFPolicy(true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Disabled {
		t.Error("Disabled should be true")
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return u
}

func TestCheckRedirect_DepthLimit(t *testing.T) {
	p := SSRFPolicy{}
	via := make([]*http.Request, 10)
	req := &http.Request{URL: mustURL(t, "https://example.com")}
	if err := p.CheckRedirect(req, via); err == nil {
		t.Error("expected error at 10 redirects")
	}
}

func TestCheckRedirect_LiteralPrivateIP(t *testing.T) {
	p := SSRFPolicy{}
	req := &http.Request{URL: mustURL(t, "http://127.0.0.1:9999/x")}
	if err := p.CheckRedirect(req, nil); err == nil {
		t.Error("expected reject for literal loopback in redirect")
	}
}

func TestCheckRedirect_AllowlistedSuffix(t *testing.T) {
	p := SSRFPolicy{AllowSuffixes: []string{"home.lan"}}
	req := &http.Request{URL: mustURL(t, "http://nas.home.lan/x")}
	if err := p.CheckRedirect(req, nil); err != nil {
		t.Errorf("expected allow for suffix-allowlisted host, got %v", err)
	}
}

func TestCheckRedirect_HostnameDeferToDialer(t *testing.T) {
	p := SSRFPolicy{}
	req := &http.Request{URL: mustURL(t, "https://example.com")}
	if err := p.CheckRedirect(req, nil); err != nil {
		t.Errorf("expected nil (defer to dialer), got %v", err)
	}
}

func TestCheckRedirect_Disabled(t *testing.T) {
	p := SSRFPolicy{Disabled: true}
	req := &http.Request{URL: mustURL(t, "http://127.0.0.1/x")}
	if err := p.CheckRedirect(req, nil); err != nil {
		t.Errorf("Disabled should allow, got %v", err)
	}
}
