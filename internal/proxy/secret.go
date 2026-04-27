package proxy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/bcrisp4/tap/internal/storage"
)

const secretKey = "proxy_hmac_secret"

// EnsureSecret returns the proxy HMAC secret from the config table,
// generating + persisting one on first call. Idempotent.
//
// On first run the secret is 32 random bytes, hex-encoded (64 chars).
// Rotating the secret (by deleting the row) invalidates browser
// caches but not the on-disk cache.
func EnsureSecret(ctx context.Context, s *storage.Store) (string, error) {
	got, err := s.GetConfig(ctx, secretKey)
	if err == nil {
		return got, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return "", fmt.Errorf("proxy: read secret: %w", err)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("proxy: generate secret: %w", err)
	}
	hexSecret := hex.EncodeToString(buf)
	if err := s.SetConfig(ctx, secretKey, hexSecret); err != nil {
		return "", fmt.Errorf("proxy: persist secret: %w", err)
	}
	return hexSecret, nil
}
