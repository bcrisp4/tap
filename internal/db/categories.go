package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrCategoryNotFound  = errors.New("category not found")
	ErrCategoryNameTaken = errors.New("category name already exists for this user")
)

type Category struct {
	ID        int64
	UserID    int64
	Name      string
	CreatedAt int64
	Position  int64
	Unread    int
}

type NewCategory struct {
	UserID    int64
	Name      string
	CreatedAt int64
}

func InsertCategory(ctx context.Context, d *sql.DB, c NewCategory) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO categories (user_id, name, created_at, position)
		VALUES (
			?, ?, ?,
			(SELECT COALESCE(MAX(position), -1) + 1 FROM categories WHERE user_id = ?)
		)
	`, c.UserID, c.Name, c.CreatedAt, c.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: categories.user_id, categories.name") {
			return 0, ErrCategoryNameTaken
		}
		return 0, fmt.Errorf("insert category: %w", err)
	}
	return res.LastInsertId()
}

func GetCategory(ctx context.Context, d *sql.DB, id, userID int64) (Category, error) {
	var c Category
	err := d.QueryRowContext(ctx,
		`SELECT id, user_id, name, created_at, position FROM categories WHERE id = ? AND user_id = ?`,
		id, userID).Scan(&c.ID, &c.UserID, &c.Name, &c.CreatedAt, &c.Position)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Category{}, ErrCategoryNotFound
		}
		return Category{}, fmt.Errorf("get category %d: %w", id, err)
	}
	return c, nil
}

func ListCategories(ctx context.Context, d *sql.DB, userID int64) ([]Category, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT c.id, c.user_id, c.name, c.created_at, c.position,
		       COUNT(CASE WHEN e.read = 0 THEN 1 END) AS unread
		FROM categories c
		LEFT JOIN subscriptions s ON s.category_id = c.id AND s.user_id = c.user_id
		LEFT JOIN entries e ON e.subscription_id = s.id
		WHERE c.user_id = ?
		GROUP BY c.id
		ORDER BY c.position ASC, c.name COLLATE NOCASE
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	var out []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.CreatedAt, &c.Position, &c.Unread); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func UpdateCategoryName(ctx context.Context, d *sql.DB, id, userID int64, name string) error {
	res, err := d.ExecContext(ctx,
		`UPDATE categories SET name = ? WHERE id = ? AND user_id = ?`,
		name, id, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed: categories.user_id, categories.name") {
			return ErrCategoryNameTaken
		}
		return fmt.Errorf("update category name %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

func DeleteCategory(ctx context.Context, d *sql.DB, id, userID int64) error {
	res, err := d.ExecContext(ctx,
		`DELETE FROM categories WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return fmt.Errorf("delete category %d: %w", id, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrCategoryNotFound
	}
	return nil
}

var ErrCategoryReorderMismatch = errors.New("category reorder ID set does not match user's categories")

// ReorderCategories rewrites the position column for every category owned by
// userID. orderedIDs must contain exactly the user's full set of category IDs
// (no duplicates, no extras, no missing). Position 0 is assigned to
// orderedIDs[0], position 1 to orderedIDs[1], and so on. Runs in a single
// transaction; on validation failure no rows change.
func ReorderCategories(ctx context.Context, d *sql.DB, userID int64, orderedIDs []int64) error {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM categories WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("list user category ids: %w", err)
	}
	have := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan id: %w", err)
		}
		have[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate ids: %w", err)
	}
	rows.Close()

	if len(orderedIDs) != len(have) {
		return ErrCategoryReorderMismatch
	}
	seen := make(map[int64]struct{}, len(orderedIDs))
	for _, id := range orderedIDs {
		if _, ok := have[id]; !ok {
			return ErrCategoryReorderMismatch
		}
		if _, dup := seen[id]; dup {
			return ErrCategoryReorderMismatch
		}
		seen[id] = struct{}{}
	}

	for i, id := range orderedIDs {
		if _, err := tx.ExecContext(ctx,
			`UPDATE categories SET position = ? WHERE id = ? AND user_id = ?`,
			int64(i), id, userID); err != nil {
			return fmt.Errorf("update position for %d: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reorder: %w", err)
	}
	return nil
}

func MarkCategoryRead(ctx context.Context, d *sql.DB, categoryID, userID int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE entries SET read = 1
		WHERE read = 0
		  AND subscription_id IN (
		      SELECT id FROM subscriptions
		      WHERE category_id = ? AND user_id = ?
		  )
	`, categoryID, userID)
	if err != nil {
		return fmt.Errorf("mark category read %d: %w", categoryID, err)
	}
	return nil
}
