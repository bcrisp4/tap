package httpclient

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseAllowedHosts(t *testing.T) {
	got, err := parseAllowedHosts("marlin-tet.ts.net, 10.0.0.0/24, ,192.168.1.0/24")
	require.NoError(t, err)
	require.Equal(t, []string{"marlin-tet.ts.net"}, got.Suffixes)
	require.Len(t, got.CIDRs, 2)
}

func TestParseAllowedHosts_BadCIDRRejected(t *testing.T) {
	_, err := parseAllowedHosts("10.0.0.0/99")
	require.Error(t, err)
}

func TestSSRF_BlocksLoopbackAndPrivate(t *testing.T) {
	cases := []string{
		"127.0.0.1:80",
		"10.0.0.5:443",
		"172.16.0.1:80",
		"192.168.1.1:443",
		"169.254.169.254:80", // link-local (cloud metadata service — important!)
		"[fe80::1]:80",       // IPv6 link-local
		"[::1]:80",           // IPv6 loopback
		"[fc00::1]:80",       // IPv6 ULA
	}
	cb := newSSRFControl(SSRFConfig{})
	for _, addr := range cases {
		err := cb("tcp", addr, nil)
		require.Error(t, err, "must block %s", addr)
	}
}

func TestSSRF_AllowsPublic(t *testing.T) {
	cb := newSSRFControl(SSRFConfig{})
	for _, addr := range []string{"8.8.8.8:443", "1.1.1.1:80", "[2606:4700::1111]:443"} {
		require.NoError(t, cb("tcp", addr, nil), "must allow %s", addr)
	}
}

func TestSSRF_AllowPrivateNetworksDisablesCheck(t *testing.T) {
	cb := newSSRFControl(SSRFConfig{AllowPrivate: true})
	require.NoError(t, cb("tcp", "10.0.0.5:80", nil))
	require.NoError(t, cb("tcp", "127.0.0.1:80", nil))
}

func TestSSRF_CIDRAllowlistBypass(t *testing.T) {
	_, cidr, _ := net.ParseCIDR("10.0.0.0/24")
	cb := newSSRFControl(SSRFConfig{
		Allowlist: AllowedHosts{CIDRs: []*net.IPNet{cidr}},
	})
	require.NoError(t, cb("tcp", "10.0.0.5:80", nil))
	require.Error(t, cb("tcp", "10.0.1.5:80", nil), "outside CIDR still blocked")
	require.Error(t, cb("tcp", "127.0.0.1:80", nil), "loopback never bypassed")
}

func TestHostnameSuffixMatch(t *testing.T) {
	a := AllowedHosts{Suffixes: []string{"marlin-tet.ts.net", "internal.example.com"}}
	require.True(t, a.MatchesHost("foo.marlin-tet.ts.net"))
	require.True(t, a.MatchesHost("marlin-tet.ts.net"))
	require.False(t, a.MatchesHost("evil.com"))
	require.False(t, a.MatchesHost("notmarlin-tet.ts.net"), "suffix must match on dot boundary")
}
