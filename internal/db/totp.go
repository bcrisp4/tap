package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type TOTPSecret struct {
	ID              int64
	UserID          int64
	SecretEncrypted []byte
	Confirmed       bool
	CreatedAt       int64
}

type RecoveryCode struct {
	ID         int64
	UserID     int64
	CodeHash   string
	ConsumedAt sql.NullInt64
}

func InsertTOTPSecret(ctx context.Context, d *sql.DB, userID int64, encrypted []byte) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO totp_secrets (user_id, secret_encrypted, confirmed, created_at)
		 VALUES (?, ?, 0, ?)
		 ON CONFLICT(user_id) DO UPDATE SET secret_encrypted=excluded.secret_encrypted,
		 confirmed=0, created_at=excluded.created_at`,
		userID, encrypted, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("insert totp secret: %w", err)
	}
	return nil
}

func GetTOTPSecret(ctx context.Context, d *sql.DB, userID int64) (TOTPSecret, error) {
	var s TOTPSecret
	err := d.QueryRowContext(ctx,
		`SELECT id, user_id, secret_encrypted, confirmed, created_at
		 FROM totp_secrets WHERE user_id = ?`, userID).
		Scan(&s.ID, &s.UserID, &s.SecretEncrypted, &s.Confirmed, &s.CreatedAt)
	if err != nil {
		return TOTPSecret{}, err
	}
	return s, nil
}

func ConfirmTOTPSecret(ctx context.Context, d *sql.DB, userID int64) error {
	_, err := d.ExecContext(ctx,
		`UPDATE totp_secrets SET confirmed = 1 WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("confirm totp secret: %w", err)
	}
	return nil
}

func DeleteTOTPSecret(ctx context.Context, d *sql.DB, userID int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM totp_secrets WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete totp secret: %w", err)
	}
	return nil
}

func InsertRecoveryCodes(ctx context.Context, d *sql.DB, userID int64, hashes []string) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("insert recovery codes: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	for _, h := range hashes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, userID, h); err != nil {
			return fmt.Errorf("insert recovery codes: %w", err)
		}
	}
	return tx.Commit()
}

// ConfirmTOTPWithRecoveryCodes atomically sets confirmed=1 on the TOTP secret
// and inserts the recovery code hashes in a single transaction. This prevents
// the failure mode where TOTP is confirmed but no recovery codes exist.
func ConfirmTOTPWithRecoveryCodes(ctx context.Context, d *sql.DB, userID int64, hashes []string) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("confirm totp with recovery codes: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.ExecContext(ctx,
		`UPDATE totp_secrets SET confirmed = 1 WHERE user_id = ?`, userID); err != nil {
		return fmt.Errorf("confirm totp secret: %w", err)
	}
	for _, h := range hashes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, userID, h); err != nil {
			return fmt.Errorf("insert recovery codes: %w", err)
		}
	}
	return tx.Commit()
}

func GetUnconsumedRecoveryCodes(ctx context.Context, d *sql.DB, userID int64) ([]RecoveryCode, error) {
	rows, err := d.QueryContext(ctx,
		`SELECT id, user_id, code_hash, consumed_at FROM recovery_codes
		 WHERE user_id = ? AND consumed_at IS NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("get recovery codes: %w", err)
	}
	defer rows.Close()
	var codes []RecoveryCode
	for rows.Next() {
		var c RecoveryCode
		if err := rows.Scan(&c.ID, &c.UserID, &c.CodeHash, &c.ConsumedAt); err != nil {
			return nil, fmt.Errorf("scan recovery code: %w", err)
		}
		codes = append(codes, c)
	}
	return codes, rows.Err()
}

func ConsumeRecoveryCode(ctx context.Context, d *sql.DB, id int64) error {
	_, err := d.ExecContext(ctx,
		`UPDATE recovery_codes SET consumed_at = ? WHERE id = ?`, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("consume recovery code: %w", err)
	}
	return nil
}

func DeleteRecoveryCodes(ctx context.Context, d *sql.DB, userID int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete recovery codes: %w", err)
	}
	return nil
}
