package auth_test

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestMintPendingToken(t *testing.T) {
	value, hash, err := auth.MintPendingToken()
	require.NoError(t, err)
	// value is 43 unpadded base64url chars (32 bytes)
	require.Len(t, value, 43)
	// hash is sha256 of the raw bytes
	raw, err := base64.RawURLEncoding.DecodeString(value)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	require.Equal(t, hex.EncodeToString(sum[:]), hash)
	// Two calls return distinct values
	v2, _, _ := auth.MintPendingToken()
	require.NotEqual(t, value, v2)
}
