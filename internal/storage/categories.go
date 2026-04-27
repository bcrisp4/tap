package storage

import (
	"context"
	"database/sql"
	"errors"
)

// Category mirrors the categories table.
type Category struct {
	ID     int64
	UserID int64
	Name   string
}

func (s *Store) ListCategories(ctx context.Context, userID int64) ([]*Category, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name FROM categories WHERE user_id = ? ORDER BY name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Category
	for rows.Next() {
		c := &Category{}
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) CreateCategory(ctx context.Context, userID int64, name string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO categories(user_id, name) VALUES(?, ?)`, userID, name)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) RenameCategory(ctx context.Context, userID, id int64, name string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE categories SET name = ? WHERE id = ? AND user_id = ?`, name, id, userID)
	if err != nil {
		return err
	}
	return rowsOrNotFound(res)
}

func (s *Store) DeleteCategory(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM categories WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return rowsOrNotFound(res)
}

// rowsOrNotFound is a small helper used by every Update/Delete in the
// package: if no rows matched, return ErrNotFound; otherwise the
// underlying error. Defined here, reused by feeds.go, entries.go, etc.
func rowsOrNotFound(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// Compile-time check that the sentinel survives errors.Is.
var _ = errors.Is(ErrNotFound, ErrNotFound)
