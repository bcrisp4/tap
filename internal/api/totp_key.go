package api

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"

	"github.com/bcrisp4/tap/internal/db"
)

const totpEncryptionKeyConfig = "totp_encryption_key"

// getTOTPEncryptionKey returns the server-side 32-byte AES key for TOTP secret encryption.
// Generated once at first use and stored as hex in the configuration table.
func getTOTPEncryptionKey(ctx context.Context, d *sql.DB) ([]byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate totp key: %w", err)
	}
	hexKey := []byte(hex.EncodeToString(raw))

	stored, err := db.SetConfigIfAbsent(ctx, d, totpEncryptionKeyConfig, hexKey)
	if err != nil {
		return nil, fmt.Errorf("totp encryption key: %w", err)
	}

	key, err := hex.DecodeString(string(stored))
	if err != nil {
		return nil, fmt.Errorf("decode totp encryption key: %w", err)
	}
	return key, nil
}
