package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type AdminMetrics struct {
	FeedsTotal      int
	FeedsOK         int
	EntriesTotal    int
	Entries24h      int
	FeedsWithErrors int
	OffendingFeeds  []string
}

// GetAdminMetrics returns instance-wide stats for the admin metric grid.
// now is injected so tests can use a fixed clock; production callers pass time.Now().
//
// Entries24h counts rows where entries.fetched_at (the time the poller wrote
// the row, not the origin-feed timestamp) is within the last 24 hours.
func GetAdminMetrics(ctx context.Context, d *sql.DB, now time.Time) (AdminMetrics, error) {
	var m AdminMetrics

	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions`).Scan(&m.FeedsTotal); err != nil {
		return m, fmt.Errorf("count feeds: %w", err)
	}
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE error_count = 0`).Scan(&m.FeedsOK); err != nil {
		return m, fmt.Errorf("count feeds ok: %w", err)
	}
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM subscriptions WHERE error_count > 0`).Scan(&m.FeedsWithErrors); err != nil {
		return m, fmt.Errorf("count feeds erroring: %w", err)
	}
	if err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM entries`).Scan(&m.EntriesTotal); err != nil {
		return m, fmt.Errorf("count entries: %w", err)
	}
	cutoff := now.Add(-24 * time.Hour).Unix()
	if err := d.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entries WHERE fetched_at >= ?`, cutoff,
	).Scan(&m.Entries24h); err != nil {
		return m, fmt.Errorf("count entries 24h: %w", err)
	}

	rows, err := d.QueryContext(ctx, `
		SELECT title FROM subscriptions
		WHERE error_count > 0
		ORDER BY error_count DESC, id ASC
		LIMIT 3`)
	if err != nil {
		return m, fmt.Errorf("query offending feeds: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return m, fmt.Errorf("scan offending feed: %w", err)
		}
		m.OffendingFeeds = append(m.OffendingFeeds, t)
	}
	if err := rows.Err(); err != nil {
		return m, fmt.Errorf("iterate offending feeds: %w", err)
	}
	return m, nil
}
