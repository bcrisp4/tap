package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// MintPendingToken returns (value, tokenHash, err) for a two-step login token.
// Same shape as MintSessionToken: value is base64url(32 random bytes),
// tokenHash is hex(sha256(those 32 bytes)) for storage.
func MintPendingToken() (value, tokenHash string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("mint pending token: %w", err)
	}
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hex.EncodeToString(sum[:]), nil
}
