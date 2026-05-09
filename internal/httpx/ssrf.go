// Package httpx provides Tap's shared HTTP client with SSRF protection
// and per-host concurrency limiting. All outbound HTTP — feed polls and
// media-proxy origin fetches — flows through one client constructed via
// NewClient.
package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// ErrSSRFBlocked is returned by AllowAddr / CheckRedirect when the policy
// rejects a destination. errors.Is can distinguish SSRF errors from other
// dial failures.
var ErrSSRFBlocked = errors.New("ssrf: destination not allowed")

// SSRFPolicy describes the destination policy applied to every outbound
// request. Disabled bypasses both the dialer check and the redirect
// re-check; AllowSuffixes match dot-boundary against URL hostnames before
// DNS; AllowCIDRs match resolved IPs after DNS.
type SSRFPolicy struct {
	Disabled      bool
	AllowSuffixes []string
	AllowCIDRs    []netip.Prefix
}

// defaultRejectCIDRs covers loopback, RFC1918, link-local, CGNAT (RFC 6598),
// the unspecified address, and IPv6 ULA. CGNAT (100.64/10) is included so a
// misconfigured Tailscale URL doesn't bypass the guard — operators on a
// tailnet must explicitly allowlist their range or hostname suffix.
var defaultRejectCIDRs = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("::/128"),
}

// AllowAddr returns nil if the resolved IP is permitted, ErrSSRFBlocked
// (wrapped with location detail) if not. Disabled and CIDR allowlist
// short-circuit the default-reject path. v4-mapped-v6 inputs are checked
// in both their mapped and unmapped form against both lists so attackers
// can't bypass with ::ffff:127.0.0.1 and operators can allow with a v4
// CIDR.
func (p SSRFPolicy) AllowAddr(addr netip.Addr) error {
	if p.Disabled {
		return nil
	}
	addrUnmapped := addr.Unmap()
	for _, cidr := range p.AllowCIDRs {
		if cidr.Contains(addr) || cidr.Contains(addrUnmapped) {
			return nil
		}
	}
	for _, reject := range defaultRejectCIDRs {
		if reject.Contains(addr) || reject.Contains(addrUnmapped) {
			return fmt.Errorf("%w: %s in %s", ErrSSRFBlocked, addr, reject)
		}
	}
	return nil
}

// AllowHostname reports whether host matches an AllowSuffixes entry
// (dot-boundary, case-insensitive, trailing dot ignored). Disabled
// implies true.
func (p SSRFPolicy) AllowHostname(host string) bool {
	if p.Disabled {
		return true
	}
	host = normaliseHost(host)
	for _, suffix := range p.AllowSuffixes {
		suffix = strings.ToLower(suffix)
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// normaliseHost lowercases and strips a trailing dot so suffix matching and
// per-host-limiter keying treat "Example.COM", "example.com", and
// "example.com." as the same hostname.
func normaliseHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// CheckRedirect is the http.Client.CheckRedirect callback. It enforces a
// 10-redirect depth limit (matches stdlib default), then for the new URL:
// suffix-allowlisted hostnames pass; literal IPs are checked against
// AllowAddr; other hostnames fall through (the dialer's ControlContext
// will re-check after DNS resolution — that is the authoritative SSRF
// gate).
func (p SSRFPolicy) CheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if p.Disabled {
		return nil
	}
	host := req.URL.Hostname()
	if p.AllowHostname(host) {
		return nil
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		return p.AllowAddr(addr)
	}
	return nil
}

// ParseSSRFPolicy parses CLI-supplied allowlist entries into an SSRFPolicy.
// Entries can be CIDR blocks (192.168.1.0/24), bare IPs (127.0.0.1, ::1),
// or hostname suffixes (marlin-tet.ts.net). Whitespace and empty entries
// are ignored. Bare IPs become /32 (IPv4) or /128 (IPv6) prefixes.
func ParseSSRFPolicy(disabled bool, entries []string) (SSRFPolicy, error) {
	p := SSRFPolicy{Disabled: disabled}
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			prefix, err := netip.ParsePrefix(e)
			if err != nil {
				return SSRFPolicy{}, fmt.Errorf("ssrf-allow: invalid CIDR %q: %w", e, err)
			}
			p.AllowCIDRs = append(p.AllowCIDRs, prefix)
			continue
		}
		if addr, err := netip.ParseAddr(e); err == nil {
			bits := 32
			if addr.Is6() {
				bits = 128
			}
			p.AllowCIDRs = append(p.AllowCIDRs, netip.PrefixFrom(addr, bits))
			continue
		}
		p.AllowSuffixes = append(p.AllowSuffixes, normaliseHost(e))
	}
	return p, nil
}
