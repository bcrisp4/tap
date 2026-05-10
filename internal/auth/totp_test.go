package auth_test

import (
	"strings"
	"testing"
	"time"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestGenerateTOTPSecret(t *testing.T) {
	s1, err := auth.GenerateTOTPSecret()
	require.NoError(t, err)
	require.NotEmpty(t, s1)
	require.True(t, isBase32(s1), "expected base32 string, got %q", s1)
	s2, err := auth.GenerateTOTPSecret()
	require.NoError(t, err)
	require.NotEqual(t, s1, s2, "two calls should return distinct secrets")
}

func TestTOTPSecretURI(t *testing.T) {
	uri := auth.TOTPSecretURI("JBSWY3DPEHPK3PXP", "Tap", "alice")
	require.True(t, strings.HasPrefix(uri, "otpauth://totp/"), "uri should start with otpauth://totp/")
	require.Contains(t, uri, "secret=JBSWY3DPEHPK3PXP")
	require.Contains(t, uri, "issuer=Tap")
}

func TestVerifyTOTP(t *testing.T) {
	secret, err := auth.GenerateTOTPSecret()
	require.NoError(t, err)

	now := time.Now().Unix()
	code := auth.GenerateTOTPCode(secret, now)

	require.True(t, auth.VerifyTOTP(secret, code), "current window code should be valid")
	require.False(t, auth.VerifyTOTP(secret, "000000"), "wrong code should be invalid")

	staleCode := auth.GenerateTOTPCode(secret, now-60)
	require.False(t, auth.VerifyTOTP(secret, staleCode), "2-window-old code should be rejected")
}

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := auth.GenerateRecoveryCodes()
	require.NoError(t, err)
	require.Len(t, codes, 8)
	seen := make(map[string]bool)
	for _, c := range codes {
		require.Len(t, c, 10, "each code should be 10 chars")
		for _, ch := range c {
			require.True(t, isRecoveryCodeChar(ch), "unexpected char %q in code %q", ch, c)
		}
		require.False(t, seen[c], "codes should be distinct")
		seen[c] = true
	}
}

func isBase32(s string) bool {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	for _, c := range s {
		if !strings.ContainsRune(alphabet, c) {
			return false
		}
	}
	return true
}

func isRecoveryCodeChar(r rune) bool {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	return strings.ContainsRune(alphabet, r)
}
