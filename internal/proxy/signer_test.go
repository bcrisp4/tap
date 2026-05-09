package proxy_test

import (
	"strings"
	"testing"

	"github.com/bcrisp4/tap/internal/proxy"
	"github.com/stretchr/testify/require"
)

var testKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes

func TestSigner_SignDeterministic(t *testing.T) {
	t.Parallel()
	s := proxy.NewSigner(testKey)
	tok1 := s.Sign("https://example.com/img.png")
	tok2 := s.Sign("https://example.com/img.png")
	require.Equal(t, tok1, tok2, "Sign must be deterministic for the same (key, URL)")
	require.Contains(t, tok1, ".", "token format is <b64url>.<b64url>")
}

func TestSigner_VerifyAcceptsValidToken(t *testing.T) {
	t.Parallel()
	s := proxy.NewSigner(testKey)
	tok := s.Sign("https://example.com/img.png")
	got, ok := s.Verify(tok)
	require.True(t, ok)
	require.Equal(t, "https://example.com/img.png", got)
}

func TestSigner_VerifyRejectsMismatchedHMAC(t *testing.T) {
	t.Parallel()
	s1 := proxy.NewSigner(testKey)
	s2 := proxy.NewSigner([]byte("ffffffffffffffffffffffffffffffff"))
	tok := s1.Sign("https://example.com/img.png")
	_, ok := s2.Verify(tok)
	require.False(t, ok)
}

func TestSigner_VerifyRejectsMalformed(t *testing.T) {
	t.Parallel()
	s := proxy.NewSigner(testKey)

	cases := []string{
		"",                              // empty
		"abc",                           // no dot
		"abc.def.ghi",                   // too many dots
		strings.Repeat("!", 10) + ".AA", // invalid base64 in URL component
		"AA." + strings.Repeat("!", 10), // invalid base64 in HMAC component
	}
	for _, tok := range cases {
		_, ok := s.Verify(tok)
		require.False(t, ok, "expected rejection for %q", tok)
	}
}

func TestSigner_RewriteImageURL(t *testing.T) {
	t.Parallel()
	s := proxy.NewSigner(testKey)
	got := s.RewriteImageURL("https://example.com/img.png")
	require.True(t, strings.HasPrefix(got, "/api/v1/proxy/"), "got: %s", got)
}
