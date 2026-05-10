package auth_test

import (
	"testing"

	"github.com/bcrisp4/tap/internal/auth"
	"github.com/stretchr/testify/require"
)

func TestEncryptDecryptTOTPSecret(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	secret := "JBSWY3DPEHPK3PXP"
	ciphertext, err := auth.EncryptTOTPSecret(key, secret)
	require.NoError(t, err)
	require.NotEmpty(t, ciphertext)

	got, err := auth.DecryptTOTPSecret(key, ciphertext)
	require.NoError(t, err)
	require.Equal(t, secret, got)
}

func TestEncryptTOTPSecret_WrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key2 {
		key2[i] = 0xFF
	}

	ct, err := auth.EncryptTOTPSecret(key1, "TESTSECRET")
	require.NoError(t, err)

	_, err = auth.DecryptTOTPSecret(key2, ct)
	require.Error(t, err, "wrong key should fail to decrypt")
}

func TestEncryptTOTPSecret_NonceRandomness(t *testing.T) {
	key := make([]byte, 32)
	secret := "SAMEPLAINTEXT"

	ct1, err := auth.EncryptTOTPSecret(key, secret)
	require.NoError(t, err)
	ct2, err := auth.EncryptTOTPSecret(key, secret)
	require.NoError(t, err)
	require.NotEqual(t, ct1, ct2, "two encryptions of the same secret should differ (random nonce)")
}
