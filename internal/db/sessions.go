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
	UserAgent         string
	Address           string
	WebAuthnChallenge []byte
}

type NewSession struct {
	UserID            int64
	TokenHash         string
	CSRFToken         string
	CreatedAt         int64
	LastSeenAt        int64
	IdleExpiresAt     int64
	AbsoluteExpiresAt int64
	UserAgent         string
	Address           string
}

// InsertSession writes a new session row.
func InsertSession(ctx context.Context, d *sql.DB, s NewSession) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO sessions
		    (user_id, token_hash, csrf_token, created_at, last_seen_at,
		     idle_expires_at, absolute_expires_at, user_agent, address)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.UserID, s.TokenHash, s.CSRFToken, s.CreatedAt, s.LastSeenAt,
		s.IdleExpiresAt, s.AbsoluteExpiresAt, s.UserAgent, s.Address)
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
		       idle_expires_at, absolute_expires_at, user_agent, address, webauthn_challenge
		FROM sessions WHERE token_hash = ? AND user_id IS NOT NULL
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.CSRFToken,
		&s.CreatedAt, &s.LastSeenAt, &s.IdleExpiresAt, &s.AbsoluteExpiresAt,
		&s.UserAgent, &s.Address, &s.WebAuthnChallenge)
	if err != nil {
		return Session{}, err
	}
	return s, nil
}

// GetSessionByID looks up a session by ID (for passkey ceremony completion).
func GetSessionByID(ctx context.Context, d *sql.DB, id int64) (Session, error) {
	var s Session
	var userID sql.NullInt64
	err := d.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, csrf_token, created_at, last_seen_at,
		       idle_expires_at, absolute_expires_at, user_agent, address, webauthn_challenge
		FROM sessions WHERE id = ?
	`, id).Scan(&s.ID, &userID, &s.TokenHash, &s.CSRFToken,
		&s.CreatedAt, &s.LastSeenAt, &s.IdleExpiresAt, &s.AbsoluteExpiresAt,
		&s.UserAgent, &s.Address, &s.WebAuthnChallenge)
	if err != nil {
		return Session{}, err
	}
	if userID.Valid {
		s.UserID = userID.Int64
	}
	return s, nil
}

// RefreshSessionIdle updates last_seen_at + idle_expires_at on a session.
func RefreshSessionIdle(ctx context.Context, d *sql.DB, id, lastSeenAt, idleExpiresAt int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE sessions SET last_seen_at = ?, idle_expires_at = ? WHERE id = ?
	`, lastSeenAt, idleExpiresAt, id)
	if err != nil {
		return fmt.Errorf("refresh session idle: %w", err)
	}
	return nil
}

// UpdateSessionCSRFToken rotates the CSRF token on a session.
func UpdateSessionCSRFToken(ctx context.Context, d *sql.DB, id int64, csrf string) error {
	_, err := d.ExecContext(ctx, `UPDATE sessions SET csrf_token = ? WHERE id = ?`, csrf, id)
	if err != nil {
		return fmt.Errorf("update csrf token: %w", err)
	}
	return nil
}

// DeleteSession removes a single session row by id.
func DeleteSession(ctx context.Context, d *sql.DB, id int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteSessionsByUserID removes every session belonging to a user.
func DeleteSessionsByUserID(ctx context.Context, d *sql.DB, userID int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("delete sessions by user: %w", err)
	}
	return nil
}

// DeleteOtherSessionsForUser removes every session belonging to a user
// except keepID.
func DeleteOtherSessionsForUser(ctx context.Context, d *sql.DB, userID, keepID int64) error {
	_, err := d.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND id <> ?`, userID, keepID)
	if err != nil {
		return fmt.Errorf("delete other sessions: %w", err)
	}
	return nil
}

// ListSessionsByUserID returns all active sessions for a user.
func ListSessionsByUserID(ctx context.Context, d *sql.DB, userID int64) ([]Session, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, user_id, token_hash, csrf_token, created_at, last_seen_at,
		       idle_expires_at, absolute_expires_at, user_agent, address, webauthn_challenge
		FROM sessions WHERE user_id = ? ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.UserID, &s.TokenHash, &s.CSRFToken,
			&s.CreatedAt, &s.LastSeenAt, &s.IdleExpiresAt, &s.AbsoluteExpiresAt,
			&s.UserAgent, &s.Address, &s.WebAuthnChallenge); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetWebAuthnChallenge stores the serialised WebAuthn session data blob on a session.
func SetWebAuthnChallenge(ctx context.Context, d *sql.DB, sessionID int64, challenge []byte) error {
	_, err := d.ExecContext(ctx,
		`UPDATE sessions SET webauthn_challenge = ? WHERE id = ?`, challenge, sessionID)
	if err != nil {
		return fmt.Errorf("set webauthn challenge: %w", err)
	}
	return nil
}

// ClearWebAuthnChallenge removes the challenge blob from a session row.
func ClearWebAuthnChallenge(ctx context.Context, d *sql.DB, sessionID int64) error {
	_, err := d.ExecContext(ctx,
		`UPDATE sessions SET webauthn_challenge = NULL WHERE id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("clear webauthn challenge: %w", err)
	}
	return nil
}

// UpgradeAnonymousSession sets user_id, token_hash, csrf_token on an anonymous
// (passkey challenge) session, completing the passkey login ceremony.
func UpgradeAnonymousSession(ctx context.Context, d *sql.DB, sessionID, userID int64,
	tokenHash, csrfToken string) error {
	_, err := d.ExecContext(ctx, `
		UPDATE sessions SET user_id = ?, token_hash = ?, csrf_token = ?,
		                    webauthn_challenge = NULL
		WHERE id = ? AND user_id IS NULL
	`, userID, tokenHash, csrfToken, sessionID)
	if err != nil {
		return fmt.Errorf("upgrade anonymous session: %w", err)
	}
	return nil
}

// InsertAnonymousSession creates a session with NULL user_id for WebAuthn challenge storage.
func InsertAnonymousSession(ctx context.Context, d *sql.DB, tokenHash, csrfToken string,
	createdAt, lastSeenAt, idleExpiresAt, absoluteExpiresAt int64) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO sessions
		    (user_id, token_hash, csrf_token, created_at, last_seen_at,
		     idle_expires_at, absolute_expires_at)
		VALUES (NULL, ?, ?, ?, ?, ?, ?)
	`, tokenHash, csrfToken, createdAt, lastSeenAt, idleExpiresAt, absoluteExpiresAt)
	if err != nil {
		return 0, fmt.Errorf("insert anonymous session: %w", err)
	}
	return res.LastInsertId()
}
