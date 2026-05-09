package httpx

import (
	"errors"
	"net/netip"
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
