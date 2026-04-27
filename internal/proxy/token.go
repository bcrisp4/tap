// Package proxy implements Tap's media proxy at /api/v1/proxy/<token>.
package proxy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
)

// EncodeToken produces the proxy token for sourceURL.
//
//	token = base64url(sourceURL) + "." + hex(hmac_sha256(secret, sourceURL))[:16]
func EncodeToken(sourceURL, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(sourceURL))
	sig := hex.EncodeToString(mac.Sum(nil))[:16]
	prefix := base64.RawURLEncoding.EncodeToString([]byte(sourceURL))
	return prefix + "." + sig
}

// DecodeToken returns the source URL the token was issued for, or an
// error if the signature doesn't verify.
func DecodeToken(token, secret string) (string, error) {
	dot := strings.IndexByte(token, '.')
	if dot < 0 || dot == len(token)-1 {
		return "", errors.New("proxy: malformed token")
	}
	prefix, sig := token[:dot], token[dot+1:]
	if len(sig) != 16 {
		return "", errors.New("proxy: bad signature length")
	}
	urlBytes, err := base64.RawURLEncoding.DecodeString(prefix)
	if err != nil {
		return "", errors.New("proxy: bad base64 prefix")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(urlBytes)
	want := hex.EncodeToString(mac.Sum(nil))[:16]
	// Compare in constant time.
	if !hmac.Equal([]byte(want), []byte(sig)) {
		return "", errors.New("proxy: signature mismatch")
	}
	return string(urlBytes), nil
}
