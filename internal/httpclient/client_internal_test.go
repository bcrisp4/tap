package httpclient

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestTransportForProxy_DoesNotCorruptBase verifies that building a
// per-feed proxy transport with no other overrides does not mutate the
// shared c.protected transport. Regression guard for the original bug
// where cloneTransport returned the shared base when both DisableHTTP2
// and AllowSelfSigned were false, then the caller mutated Proxy on it.
func TestTransportForProxy_DoesNotCorruptBase(t *testing.T) {
	c := NewClient(Config{Timeout: 1 * time.Second, MaxBodyBytes: 1024})
	require.Nil(t, c.protected.Proxy, "precondition")

	_, err := c.transportForProxy("http://proxy.example.com:8080", &Options{})
	require.NoError(t, err)

	require.Nil(t, c.protected.Proxy, "shared base transport must not have Proxy mutated")
}

// TestTransportForProxy_CacheKeyIsolatesTLSOption verifies that two
// requests with the same proxy URL but different AllowSelfSigned
// settings get distinct transports — otherwise the second request
// would silently inherit the first's InsecureSkipVerify value.
func TestTransportForProxy_CacheKeyIsolatesTLSOption(t *testing.T) {
	c := NewClient(Config{Timeout: 1 * time.Second, MaxBodyBytes: 1024})

	rt1, err := c.transportForProxy("http://proxy.example.com:8080", &Options{AllowSelfSigned: true})
	require.NoError(t, err)
	rt2, err := c.transportForProxy("http://proxy.example.com:8080", &Options{AllowSelfSigned: false})
	require.NoError(t, err)
	require.NotSame(t, rt1, rt2, "AllowSelfSigned=true and =false must not share a cached transport")

	rt3, err := c.transportForProxy("http://proxy.example.com:8080", &Options{AllowSelfSigned: true})
	require.NoError(t, err)
	require.Same(t, rt1, rt3, "same key should hit the cache")
}

// TestTransportForProxy_CacheKeyIsolatesHTTP2Option mirrors the TLS
// guard for DisableHTTP2.
func TestTransportForProxy_CacheKeyIsolatesHTTP2Option(t *testing.T) {
	c := NewClient(Config{Timeout: 1 * time.Second, MaxBodyBytes: 1024})

	rt1, err := c.transportForProxy("http://proxy.example.com:8080", &Options{DisableHTTP2: true})
	require.NoError(t, err)
	rt2, err := c.transportForProxy("http://proxy.example.com:8080", &Options{DisableHTTP2: false})
	require.NoError(t, err)
	require.NotSame(t, rt1, rt2)
}
