package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// NewTombstone is the write shape for a tombstone row.
type NewTombstone struct {
	SubscriptionID int64
	EntryHash      string
	DeletedAt      int64 // unix seconds
}

// InsertTombstones bulk-inserts tombstone rows within the caller's transaction.
// ON CONFLICT DO NOTHING — existing tombstones are kept as-is (idempotent).
func InsertTombstones(ctx context.Context, tx *sql.Tx, rows []NewTombstone) error {
	for _, r := range rows {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO tombstones (subscription_id, entry_hash, deleted_at)
			VALUES (?, ?, ?)
			ON CONFLICT (subscription_id, entry_hash) DO NOTHING
		`, r.SubscriptionID, r.EntryHash, r.DeletedAt); err != nil {
			return fmt.Errorf("insert tombstone (sub=%d hash=%s): %w",
				r.SubscriptionID, r.EntryHash, err)
		}
	}
	return nil
}

// IsTombstoned returns true if (subscriptionID, entryHash) exists in tombstones.
func IsTombstoned(ctx context.Context, d *sql.DB, subscriptionID int64, entryHash string) (bool, error) {
	var exists int
	err := d.QueryRowContext(ctx, `
		SELECT 1 FROM tombstones
		WHERE subscription_id = ? AND entry_hash = ?
		LIMIT 1
	`, subscriptionID, entryHash).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check tombstone (sub=%d hash=%s): %w",
			subscriptionID, entryHash, err)
	}
	return true, nil
}
