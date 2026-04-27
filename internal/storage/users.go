package storage

import (
	"context"
	"database/sql"
	"errors"
)

// User mirrors the users table.
type User struct {
	ID             int64
	Username       string
	Theme          string
	Font           string
	EntriesPerPage int
	DefaultSort    string
	DefaultOrder   string
	CreatedAt      int64
}

// GetUser returns the user with the given id, or ErrNotFound.
func (s *Store) GetUser(ctx context.Context, id int64) (*User, error) {
	u := &User{}
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, theme, font, entries_per_page,
		       default_sort, default_order, created_at
		FROM users WHERE id = ?`, id).Scan(
		&u.ID, &u.Username, &u.Theme, &u.Font, &u.EntriesPerPage,
		&u.DefaultSort, &u.DefaultOrder, &u.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// CreateUser inserts a new user with the given username and returns
// the new id. The username UNIQUE constraint surfaces as a SQL error.
func (s *Store) CreateUser(ctx context.Context, username string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO users(username) VALUES(?)`, username)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
