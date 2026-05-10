package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // TOTP (RFC 6238) mandates SHA-1
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

// GenerateTOTPSecret returns a base32-encoded 20-byte random secret.
func GenerateTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate totp secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

// TOTPSecretURI returns the otpauth://totp/... URI for QR code display.
func TOTPSecretURI(secret, issuer, accountName string) string {
	label := url.PathEscape(issuer + ":" + accountName)
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", "6")
	v.Set("period", "30")
	return "otpauth://totp/" + label + "?" + v.Encode()
}

// GenerateTOTPCode generates the 6-digit TOTP code for a given unix timestamp.
// Exported so tests can generate known codes for VerifyTOTP.
func GenerateTOTPCode(secret string, unixTime int64) string {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(secret))
	if err != nil {
		return ""
	}
	counter := uint64(math.Floor(float64(unixTime) / 30))
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	mac := hmac.New(sha1.New, key) //nolint:gosec // RFC 6238 mandates SHA-1
	_, _ = mac.Write(buf)
	h := mac.Sum(nil)
	offset := h[len(h)-1] & 0x0f
	code := (int(h[offset]&0x7f)<<24 |
		int(h[offset+1])<<16 |
		int(h[offset+2])<<8 |
		int(h[offset+3])) % 1_000_000
	return fmt.Sprintf("%06d", code)
}

// VerifyTOTP validates a 6-digit code against the TOTP secret.
// Accepts the current window ±1 (one step of clock-drift tolerance).
func VerifyTOTP(secret, code string) bool {
	now := time.Now().Unix()
	for _, offset := range []int64{-1, 0, 1} {
		if GenerateTOTPCode(secret, now+offset*30) == code {
			return true
		}
	}
	return false
}

// recoveryAlphabet excludes 0, O, 1, I to prevent misreading.
const recoveryAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GenerateRecoveryCodes returns 8 cryptographically random 10-char codes.
func GenerateRecoveryCodes() ([]string, error) {
	codes := make([]string, 8)
	for i := range codes {
		buf := make([]byte, 10)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("generate recovery codes: %w", err)
		}
		var sb strings.Builder
		for _, b := range buf {
			sb.WriteByte(recoveryAlphabet[int(b)%len(recoveryAlphabet)])
		}
		codes[i] = sb.String()
	}
	return codes, nil
}
