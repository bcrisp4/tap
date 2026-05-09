package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrSubscriptionExists wraps the underlying SQLite UNIQUE-constraint failure
// on subscriptions.feed_url. Use errors.Is(err, ErrSubscriptionExists) in
// callers to map the duplicate case to a 409 without coupling them to the
// SQLite driver.
var ErrSubscriptionExists = errors.New("subscription with this feed_url already exists")

type Subscription struct {
	ID           int64
	Title        string
	FeedURL      string
	SiteURL      sql.NullString
	LastPollAt   sql.NullInt64
	NextPollAt   int64
	ETag         sql.NullString
	LastModified sql.NullString
	ErrorCount   int
	LastError    sql.NullString
	CreatedAt    int64
}

type NewSubscription struct {
	Title    string
	FeedURL  string
	SiteURL  string
	NextPoll int64
	Created  int64
}

type DueSubscription struct {
	ID           int64
	FeedURL      string
	ETag         sql.NullString
	LastModified sql.NullString
}

// PollResult and UpdateAfterPoll live in entries.go because they reference
// db.NewEntry, which is defined there. Keeping them together avoids a forward
// reference and lets Task 2.5 commit cleanly without depending on Task 2.6.

func InsertSubscription(ctx context.Context, d *sql.DB, s NewSubscription) (int64, error) {
	res, err := d.ExecContext(ctx, `
		INSERT INTO subscriptions (title, feed_url, site_url, next_poll_at, created_at)
		VALUES (?, ?, NULLIF(?, ''), ?, ?)
	`, s.Title, s.FeedURL, s.SiteURL, s.NextPoll, s.Created)
	if err != nil {
		// modernc.org/sqlite reports unique violations through the standard
		// SQLite error text. We match on substring rather than the typed
		// driver error so the api package stays driver-agnostic.
		if strings.Contains(err.Error(), "UNIQUE constraint failed: subscriptions.feed_url") {
			return 0, ErrSubscriptionExists
		}
		return 0, fmt.Errorf("insert subscription: %w", err)
	}
	return res.LastInsertId()
}

func GetSubscription(ctx context.Context, d *sql.DB, id int64) (Subscription, error) {
	var s Subscription
	err := d.QueryRowContext(ctx, `
		SELECT id, title, feed_url, site_url, last_poll_at, next_poll_at,
		       etag, last_modified, error_count, last_error, created_at
		FROM subscriptions WHERE id = ?
	`, id).Scan(&s.ID, &s.Title, &s.FeedURL, &s.SiteURL, &s.LastPollAt, &s.NextPollAt,
		&s.ETag, &s.LastModified, &s.ErrorCount, &s.LastError, &s.CreatedAt)
	if err != nil {
		return Subscription{}, fmt.Errorf("get subscription %d: %w", id, err)
	}
	return s, nil
}

func ListSubscriptions(ctx context.Context, d *sql.DB) ([]Subscription, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, title, feed_url, site_url, last_poll_at, next_poll_at,
		       etag, last_modified, error_count, last_error, created_at
		FROM subscriptions ORDER BY title COLLATE NOCASE
	`)
	if err != nil {
		return nil, fmt.Errorf("list subscriptions: %w", err)
	}
	defer rows.Close()

	var out []Subscription
	for rows.Next() {
		var s Subscription
		if err := rows.Scan(&s.ID, &s.Title, &s.FeedURL, &s.SiteURL, &s.LastPollAt, &s.NextPollAt,
			&s.ETag, &s.LastModified, &s.ErrorCount, &s.LastError, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan subscription: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func DeleteSubscription(ctx context.Context, d *sql.DB, id int64) error {
	_, err := d.ExecContext(ctx, "DELETE FROM subscriptions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete subscription %d: %w", id, err)
	}
	return nil
}

// ListDuePolls returns subscriptions whose next_poll_at is <= now, capped at limit.
// The caller is responsible for excluding currently in-flight subscriptions.
func ListDuePolls(ctx context.Context, d *sql.DB, now int64, limit int) ([]DueSubscription, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT id, feed_url, etag, last_modified
		FROM subscriptions
		WHERE next_poll_at <= ?
		ORDER BY next_poll_at
		LIMIT ?
	`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("list due polls: %w", err)
	}
	defer rows.Close()

	var out []DueSubscription
	for rows.Next() {
		var s DueSubscription
		if err := rows.Scan(&s.ID, &s.FeedURL, &s.ETag, &s.LastModified); err != nil {
			return nil, fmt.Errorf("scan due poll: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpdateAfterError records a poll failure and pushes next_poll_at out.
func UpdateAfterError(ctx context.Context, d *sql.DB, subID int64, errMsg string, nextPollAt int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE subscriptions
		SET error_count = error_count + 1,
		    last_error  = ?,
		    next_poll_at = ?
		WHERE id = ?
	`, errMsg, nextPollAt, subID)
	if err != nil {
		return fmt.Errorf("record poll error: %w", err)
	}
	return nil
}

// UpdateAfterNotModified bumps timestamps without inserting anything (304 path).
func UpdateAfterNotModified(ctx context.Context, d *sql.DB, subID int64, nowUnix, nextPollAt int64) error {
	_, err := d.ExecContext(ctx, `
		UPDATE subscriptions
		SET last_poll_at = ?, next_poll_at = ?, error_count = 0, last_error = NULL
		WHERE id = ?
	`, nowUnix, nextPollAt, subID)
	if err != nil {
		return fmt.Errorf("record 304: %w", err)
	}
	return nil
}

// QueryVelocity returns the rolling 7-day entries-per-day rate × 100 for a
// single subscription. Returns 0 if the subscription has no entries in the
// window. The caller is responsible for calling this inside the same logical
// poll boundary so the count reflects the post-insert state.
func QueryVelocity(ctx context.Context, d *sql.DB, subID int64, now time.Time) (int, error) {
	cutoff := now.Add(-7 * 24 * time.Hour).Unix()
	var velocity int
	err := d.QueryRowContext(ctx, `
		SELECT COUNT(*) * 100 / 7
		FROM entries
		WHERE subscription_id = ? AND published_at >= ?
	`, subID, cutoff).Scan(&velocity)
	if err != nil {
		return 0, fmt.Errorf("query velocity: %w", err)
	}
	return velocity, nil
}
