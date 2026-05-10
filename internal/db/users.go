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
