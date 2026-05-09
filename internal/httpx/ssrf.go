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

var ErrSSRFBlocked = errors.New("ssrf: destination not allowed")

type SSRFPolicy struct {
	Disabled      bool
	AllowSuffixes []string
	AllowCIDRs    []netip.Prefix
}

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

func (p SSRFPolicy) AllowHostname(host string) bool {
	if p.Disabled {
		return true
	}
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	for _, suffix := range p.AllowSuffixes {
		suffix = strings.ToLower(suffix)
		if host == suffix || strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
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
		p.AllowSuffixes = append(p.AllowSuffixes, strings.ToLower(e))
	}
	return p, nil
}
