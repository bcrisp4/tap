package proxy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testSecret = "deadbeef"

func TestToken_RoundTrip(t *testing.T) {
	url := "https://cdn.example.com/img/x.png"
	tok := EncodeToken(url, testSecret)
	require.Contains(t, tok, ".")

	got, err := DecodeToken(tok, testSecret)
	require.NoError(t, err)
	require.Equal(t, url, got)
}

func TestToken_TamperedURLRejected(t *testing.T) {
	tok := EncodeToken("https://cdn.example.com/a.png", testSecret)
	parts := strings.SplitN(tok, ".", 2)
	require.Len(t, parts, 2)
	// Replace the URL prefix with a base64 of a different URL but
	// leave the original signature.
	bad := EncodeToken("https://cdn.evil.com/b.png", testSecret)
	badParts := strings.SplitN(bad, ".", 2)
	stitched := badParts[0] + "." + parts[1]
	_, err := DecodeToken(stitched, testSecret)
	require.Error(t, err)
}

func TestToken_TamperedSignatureRejected(t *testing.T) {
	tok := EncodeToken("https://x", testSecret)
	parts := strings.SplitN(tok, ".", 2)
	require.Len(t, parts, 2)
	tampered := parts[0] + "." + strings.Repeat("0", len(parts[1]))
	_, err := DecodeToken(tampered, testSecret)
	require.Error(t, err)
}

func TestToken_DifferentSecretsRejected(t *testing.T) {
	tok := EncodeToken("https://x", "secret-a")
	_, err := DecodeToken(tok, "secret-b")
	require.Error(t, err)
}

func TestToken_SignatureLength16Hex(t *testing.T) {
	tok := EncodeToken("https://x", testSecret)
	parts := strings.SplitN(tok, ".", 2)
	require.Len(t, parts[1], 16, "spec: hex(hmac)[:16]")
}
