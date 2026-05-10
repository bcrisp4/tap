package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// EncryptTOTPSecret encrypts a base32 TOTP secret using AES-256-GCM.
// The 12-byte nonce is prepended to the ciphertext in the returned slice.
// key must be exactly 32 bytes.
func EncryptTOTPSecret(key []byte, secret string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encrypt totp secret: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encrypt totp secret: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encrypt totp secret: read nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(secret), nil)
	return ciphertext, nil
}

// DecryptTOTPSecret reverses EncryptTOTPSecret.
// Returns an error if the key is wrong or the ciphertext is malformed.
func DecryptTOTPSecret(key []byte, ciphertext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("decrypt totp secret: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("decrypt totp secret: new gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("decrypt totp secret: ciphertext too short")
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt totp secret: %w", err)
	}
	return string(plaintext), nil
}
