package proxy

import (
	"crypto/sha256"
	"encoding/hex"
)

// etagOf returns a strong ETag derived from the body bytes. Used as a
// fallback when the origin response has no ETag header, so 304
// short-circuits still work for SPA pre-fetched content.
func etagOf(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:])[:32] + `"`
}
