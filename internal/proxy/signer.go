// Package proxy implements the internal media proxy: signed-token URLs,
// filesystem cache with JSON sidecar metadata, MIME validation, and the
// HTTP handler that ties them together. See docs/specs/2026-05-09-m3-media-proxy.md.
package proxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

// Signer signs and verifies media-proxy tokens with HMAC-SHA256.
// The HMAC is truncated to 16 bytes (128 bits) — well above any reasonable
// forge cost for an internal-only proxy and keeps tokens compact.
type Signer struct {
	key []byte
}

const hmacBytes = 16
const proxySigningKeyMinBytes = 32

// NewSigner copies the key. Callers may safely reuse the underlying slice.
// Panics if key is shorter than 32 bytes — the production caller in
// cmd/tap/main.go generates a 32-byte key from crypto/rand, and shorter keys
// break the security model.
func NewSigner(key []byte) *Signer {
	if len(key) < proxySigningKeyMinBytes {
		panic("proxy.NewSigner: key must be at least 32 bytes")
	}
	cp := make([]byte, len(key))
	copy(cp, key)
	return &Signer{key: cp}
}

// Sign returns "<b64url(rawURL)>.<b64url(hmac[:16])>". Deterministic per (key, url).
func (s *Signer) Sign(rawURL string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(rawURL))
	sum := mac.Sum(nil)[:hmacBytes]

	enc := base64.RawURLEncoding
	return enc.EncodeToString([]byte(rawURL)) + "." + enc.EncodeToString(sum)
}

// RewriteImageURL returns "/api/v1/proxy/<token>" — the exact form the
// processor injects into <img src> attributes.
func (s *Signer) RewriteImageURL(rawURL string) string {
	return "/api/v1/proxy/" + s.Sign(rawURL)
}

// Verify parses tok, recomputes the HMAC, and constant-time-compares.
// Returns the original URL on success.
func (s *Signer) Verify(tok string) (string, bool) {
	dot := strings.IndexByte(tok, '.')
	if dot <= 0 || dot == len(tok)-1 {
		return "", false
	}
	if strings.Contains(tok[dot+1:], ".") {
		return "", false
	}
	enc := base64.RawURLEncoding
	rawURL, err := enc.DecodeString(tok[:dot])
	if err != nil {
		return "", false
	}
	gotMAC, err := enc.DecodeString(tok[dot+1:])
	if err != nil {
		return "", false
	}

	mac := hmac.New(sha256.New, s.key)
	mac.Write(rawURL)
	wantMAC := mac.Sum(nil)[:hmacBytes]

	if !hmac.Equal(gotMAC, wantMAC) {
		return "", false
	}
	return string(rawURL), true
}
