package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// GetConfig returns the bytes stored under key. ok=false means the row is absent;
// (nil, false, nil) is the canonical "not found" tuple.
func GetConfig(ctx context.Context, d *sql.DB, key string) (value []byte, ok bool, err error) {
	row := d.QueryRowContext(ctx, `SELECT value FROM configuration WHERE key = ?`, key)
	var v []byte
	if err := row.Scan(&v); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("scan configuration[%s]: %w", key, err)
	}
	return v, true, nil
}

// SetConfigIfAbsent inserts (key, value) if the row doesn't exist, then returns
// the row's actual value (so callers see the existing value when present).
// Idempotent: a second call with the same key is a no-op.
func SetConfigIfAbsent(ctx context.Context, d *sql.DB, key string, value []byte) ([]byte, error) {
	if _, err := d.ExecContext(ctx,
		`INSERT INTO configuration (key, value) VALUES (?, ?) ON CONFLICT(key) DO NOTHING`,
		key, value); err != nil {
		return nil, fmt.Errorf("insert configuration[%s]: %w", key, err)
	}
	got, ok, err := GetConfig(ctx, d, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("configuration[%s] missing after upsert", key)
	}
	return got, nil
}
