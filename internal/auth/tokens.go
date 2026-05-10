package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// sessionTokenBytes is the size of the random portion behind every session
// cookie. 32 bytes (256 bits) is the standard "unguessable" choice and the
// industry default for opaque session tokens.
const sessionTokenBytes = 32

// MintSessionToken returns the cookie value and its sha256-hex hash.
//   - cookieValue: base64url(random 32 bytes), unpadded — what goes in the
//     Set-Cookie header.
//   - tokenHash:   hex(sha256(those 32 bytes)) — what gets stored on
//     sessions.token_hash. A DB read therefore never yields a live cookie.
func MintSessionToken() (cookieValue, tokenHash string, err error) {
	raw := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("mint session token: %w", err)
	}
	sum := sha256.Sum256(raw)
	return base64.RawURLEncoding.EncodeToString(raw), hex.EncodeToString(sum[:]), nil
}

// csrfTokenBytes is the random size behind a CSRF token. 32 bytes matches
// the session token; the CSRF token is also unguessable but doesn't need
// to be hashed at rest because it's not a credential — leaking it grants
// CSRF bypass for the bound session, not session takeover.
const csrfTokenBytes = 32

// MintCSRFToken returns base64url(random 32 bytes), unpadded.
func MintCSRFToken() (string, error) {
	raw := make([]byte, csrfTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("mint csrf token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
