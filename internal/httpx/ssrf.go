// Package httpx provides Tap's shared HTTP client with SSRF protection
// and per-host concurrency limiting. All outbound HTTP — feed polls and
// media-proxy origin fetches — flows through one client constructed via
// NewClient.
package httpx

import (
	"errors"
	"fmt"
	"net/netip"
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
	for _, cidr := range p.AllowCIDRs {
		if cidr.Contains(addr) {
			return nil
		}
	}
	addrUnmapped := addr.Unmap()
	for _, reject := range defaultRejectCIDRs {
		if reject.Contains(addr) || reject.Contains(addrUnmapped) {
			return fmt.Errorf("%w: %s in %s", ErrSSRFBlocked, addr, reject)
		}
	}
	return nil
}
