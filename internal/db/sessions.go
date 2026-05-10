package db

import (
	"context"
	"database/sql"
	"fmt"
)

type Session struct {
	ID                int64
	UserID            int64
	TokenHash         string
	CSRFToken         string
	CreatedAt         int64
	LastSeenAt        int64
	IdleExpiresAt     int64
	AbsoluteExpiresAt int64
}

type NewSession struct {
	UserID            int64
	TokenHash         string
	CSRFToken         string
	CreatedAt         int64
	LastSeenAt        int64
	IdleExpiresAt     int64
	AbsoluteExpiresAt int64
}

// InsertSession writes a new session row.
func InsertSession(ctx context.Context, d *sql.DB, s NewSession) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO sessions
		    (user_id, token_hash, csrf_token, created_at, last_seen_at,
		     idle_expires_at, absolute_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, s.UserID, s.TokenHash, s.CSRFToken, s.CreatedAt, s.LastSeenAt,
		s.IdleExpiresAt, s.AbsoluteExpiresAt)
	if err != nil {
		return 0, fmt.Errorf("insert session: %w", err)
	}
	return res.LastInsertId()
}

// GetSessionByTokenHash looks up a session by its sha256-hex token hash.
// Returns sql.ErrNoRows if no row matches; the middleware maps that to a
// generic 401 invalid_session response so this function does not.
func GetSessionByTokenHash(ctx context.Context, d *sql.DB, tokenHash string) (Session, error) {
	var s Session
	err := d.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, csrf_token, created_at, last_seen_at,
		       idle_expires_at, absolute_expires_at
		FROM sessions WHERE token_hash = ?
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.CSRFToken,
		&s.CreatedAt, &s.LastSeenAt, &s.IdleExpiresAt, &s.AbsoluteExpiresAt)
	if err != nil {
		return Session{}, err
	}
	return s, nil
}

// RefreshSessionIdle updates last_seen_at + idle_expires_at on a session.
// Concurrent refreshes converge — SQLite's WAL serialises writes.
func RefreshSessionIdle(ctx context.Context, d *sql.DB, id, lastSeenAt, idleExpiresAt int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE sessions SET last_seen_at = ?, idle_expires_at = ? WHERE id = ?
	`, lastSeenAt, idleExpiresAt, id)
	if err != nil {
		return fmt.Errorf("refresh session idle: %w", err)
	}
	return nil
}

// UpdateSessionCSRFToken rotates the CSRF token on a session. PATCH
// /me/password is the only call site; tokens otherwise live for the
// session's full lifetime.
func UpdateSessionCSRFToken(ctx context.Context, d *sql.DB, id int64, csrf string) error {
	_, err := d.ExecContext(ctx, `UPDATE sessions SET csrf_token = ? WHERE id = ?`, csrf, id)
	if err != nil {
		return fmt.Errorf("update csrf token: %w", err)
	}
	return nil
}

// DeleteSession removes a single session row by id. Used by logout and by
// the middleware when an absolute-expired session is encountered.
func DeleteSession(ctx context.Context, d *sql.DB, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteSessionsByUserID removes every session belonging to a user. Used by
// `tap admin passwd` (admin reset implies the user is locked out).
func DeleteSessionsByUserID(ctx context.Context, d *sql.DB, userID int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete sessions by user: %w", err)
	}
	return nil
}

// DeleteOtherSessionsForUser removes every session belonging to a user
// except keepID. Used by PATCH /me/password — the user-initiated password
// change keeps the current session active while invalidating any others.
func DeleteOtherSessionsForUser(ctx context.Context, d *sql.DB, userID, keepID int64) error {
	_, err := d.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND id <> ?`, userID, keepID)
	if err != nil {
		return fmt.Errorf("delete other sessions: %w", err)
	}
	return nil
}
