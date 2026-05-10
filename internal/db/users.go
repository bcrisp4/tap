package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ErrUserExists wraps the underlying SQLite UNIQUE-constraint failure on
// users.username. Use errors.Is(err, ErrUserExists) in callers to map the
// duplicate case without coupling to the SQLite driver's error text.
var ErrUserExists = errors.New("user with this username already exists")

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	Role         string
	CreatedAt    int64
	DisabledAt   sql.NullInt64
}

type NewUser struct {
	Username     string
	PasswordHash string
	Role         string // "admin" or "user"
	CreatedAt    int64
}

// InsertUser writes a new user. Returns ErrUserExists on duplicate username.
func InsertUser(ctx context.Context, d *sql.DB, u NewUser) (int64, error) {
	res, err := d.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		u.Username, u.PasswordHash, u.Role, u.CreatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: users.username") {
			return 0, ErrUserExists
		}
		return 0, fmt.Errorf("insert user: %w", err)
	}
	return res.LastInsertId()
}

// GetUserByUsername returns the user with the given username. Returns
// sql.ErrNoRows if no row matches; the auth layer maps that to a generic
// invalid_credentials response so this function does not.
func GetUserByUsername(ctx context.Context, d *sql.DB, username string) (User, error) {
	var u User
	err := d.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, created_at, disabled_at
		FROM users WHERE username = ?
	`, username).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.DisabledAt)
	if err != nil {
		return User{}, err // bare-return so errors.Is(err, sql.ErrNoRows) works upstream
	}
	return u, nil
}

// GetUserByID returns the user with the given primary-key id. Returns
// sql.ErrNoRows if no such user.
func GetUserByID(ctx context.Context, d *sql.DB, id int64) (User, error) {
	var u User
	err := d.QueryRowContext(ctx, `
		SELECT id, username, password_hash, role, created_at, disabled_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &u.CreatedAt, &u.DisabledAt)
	if err != nil {
		return User{}, err
	}
	return u, nil
}

// UpdatePasswordHash overwrites the password_hash on a user. The caller is
// responsible for any session-invalidation policy (PATCH /me/password
// keeps the current session and deletes others; tap admin passwd deletes
// every session for the user).
func UpdatePasswordHash(ctx context.Context, d *sql.DB, id int64, hash string) error {
	res, err := d.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	if err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DisableUser sets disabled_at on a user. The login path checks this column
// and rejects login for disabled accounts; existing sessions remain valid
// until they expire (M7 closes that gap with the admin-reset path).
func DisableUser(ctx context.Context, d *sql.DB, id, disabledAt int64) error {
	res, err := d.ExecContext(ctx, `UPDATE users SET disabled_at = ? WHERE id = ?`, disabledAt, id)
	if err != nil {
		return fmt.Errorf("disable user: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CountUsers returns the number of rows in the users table. Used by the
// env-var bootstrap path to decide whether to create the first admin.
func CountUsers(ctx context.Context, d *sql.DB) (int, error) {
	var n int
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return n, nil
}
