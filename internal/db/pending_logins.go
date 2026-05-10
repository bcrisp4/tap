package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type PendingLogin struct {
	ID        int64
	TokenHash string
	UserID    int64
	CreatedAt int64
	ExpiresAt int64
}

func InsertPendingLogin(ctx context.Context, d *sql.DB, userID int64, tokenHash string, expiresAt int64) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO pending_logins (token_hash, user_id, created_at, expires_at)
		 VALUES (?, ?, ?, ?)`,
		tokenHash, userID, time.Now().Unix(), expiresAt)
	if err != nil {
		return fmt.Errorf("insert pending login: %w", err)
	}
	return nil
}

func GetPendingLoginByTokenHash(ctx context.Context, d *sql.DB, tokenHash string) (PendingLogin, error) {
	var p PendingLogin
	err := d.QueryRowContext(ctx,
		`SELECT id, token_hash, user_id, created_at, expires_at
		 FROM pending_logins WHERE token_hash = ?`, tokenHash).
		Scan(&p.ID, &p.TokenHash, &p.UserID, &p.CreatedAt, &p.ExpiresAt)
	if err != nil {
		return PendingLogin{}, err
	}
	return p, nil
}

func DeletePendingLogin(ctx context.Context, d *sql.DB, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM pending_logins WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete pending login: %w", err)
	}
	return nil
}

// DeleteExpiredPendingLogins removes all pending_logins rows whose expires_at <= now.
// Called on login handler entry to prune stale rows without a separate ticker.
func DeleteExpiredPendingLogins(ctx context.Context, d *sql.DB, now int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM pending_logins WHERE expires_at <= ?`, now)
	if err != nil {
		return fmt.Errorf("delete expired pending logins: %w", err)
	}
	return nil
}
