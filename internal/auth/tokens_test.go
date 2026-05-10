package auth

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMintSessionTokenShape(t *testing.T) {
	t.Parallel()
	cookie, hash, err := MintSessionToken()
	require.NoError(t, err)

	// 32 random bytes → 43 base64url chars (unpadded).
	require.Len(t, cookie, 43, "cookie value")
	raw, err := base64.RawURLEncoding.DecodeString(cookie)
	require.NoError(t, err)
	require.Len(t, raw, 32, "underlying random bytes")

	// hash is sha256-hex of the underlying random bytes.
	sum := sha256.Sum256(raw)
	require.Equal(t, hex.EncodeToString(sum[:]), hash)
	require.Len(t, hash, 64)
}

func TestMintSessionTokenDistinctness(t *testing.T) {
	t.Parallel()
	a, _, err := MintSessionToken()
	require.NoError(t, err)
	b, _, err := MintSessionToken()
	require.NoError(t, err)
	require.NotEqual(t, a, b, "two mints should produce distinct cookies")
}

func TestMintCSRFToken(t *testing.T) {
	t.Parallel()
	a, err := MintCSRFToken()
	require.NoError(t, err)
	require.Len(t, a, 43)

	b, err := MintCSRFToken()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
}
