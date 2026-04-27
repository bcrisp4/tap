package httpclient

import (
	"fmt"
	"net"
	"strings"
	"syscall"
)

// AllowedHosts is the parsed form of TAP_ALLOWED_HOSTS:
// hostname-suffix matches bypass at request time; CIDR matches bypass
// at dial time.
type AllowedHosts struct {
	Suffixes []string
	CIDRs    []*net.IPNet
}

// MatchesHost reports whether host (no port) is on the suffix
// allowlist. Match is on dot boundary so "marlin-tet.ts.net" only
// matches "*.marlin-tet.ts.net" and the bare suffix itself.
func (a AllowedHosts) MatchesHost(host string) bool {
	host = strings.ToLower(host)
	for _, s := range a.Suffixes {
		s = strings.ToLower(s)
		if host == s || strings.HasSuffix(host, "."+s) {
			return true
		}
	}
	return false
}

func (a AllowedHosts) ipInCIDR(ip net.IP) bool {
	for _, c := range a.CIDRs {
		if c.Contains(ip) {
			return true
		}
	}
	return false
}

// parseAllowedHosts splits a comma-separated string into hostname
// suffixes and CIDR blocks. Empty entries are skipped. Unrecognised
// entries (not a CIDR and contain no dot) are also skipped — they're
// likely typos. Returns an error if a CIDR-shaped entry fails to parse.
func parseAllowedHosts(s string) (AllowedHosts, error) {
	var out AllowedHosts
	for _, raw := range strings.Split(s, ",") {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if strings.Contains(entry, "/") {
			_, cidr, err := net.ParseCIDR(entry)
			if err != nil {
				return AllowedHosts{}, fmt.Errorf("ssrf: parse CIDR %q: %w", entry, err)
			}
			out.CIDRs = append(out.CIDRs, cidr)
			continue
		}
		if strings.Contains(entry, ".") {
			out.Suffixes = append(out.Suffixes, entry)
		}
	}
	return out, nil
}

// SSRFConfig configures the dialer control callback.
type SSRFConfig struct {
	AllowPrivate bool         // if true, no IP is rejected
	Allowlist    AllowedHosts // CIDRs that bypass the private-network check
}

// newSSRFControl returns a Dialer.Control function that rejects
// loopback / private / link-local / ULA addresses unless config
// permits them.
func newSSRFControl(cfg SSRFConfig) func(network, addr string, c syscall.RawConn) error {
	return func(_ string, addr string, _ syscall.RawConn) error {
		if cfg.AllowPrivate {
			return nil
		}
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("ssrf: split host/port %q: %w", addr, err)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("ssrf: not an IP literal %q (DNS rebinding?)", host)
		}
		if ipIsBlocked(ip) {
			if cfg.Allowlist.ipInCIDR(ip) && !ip.IsLoopback() {
				return nil
			}
			return fmt.Errorf("ssrf: blocked private/loopback/link-local address %s", ip)
		}
		return nil
	}
}

// ipIsBlocked reports whether ip is in any of:
// loopback, private (RFC1918 / RFC4193), link-local, unspecified,
// multicast, or interface-local.
func ipIsBlocked(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast()
}
