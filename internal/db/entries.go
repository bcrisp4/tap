package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/bcrisp4/tap/internal/cadence"
)

type NewEntry struct {
	Hash        string
	Title       string
	Author      string
	URL         string
	Content     string
	PublishedAt int64
}

// PollResult is the success result of a feed fetch+parse worth committing.
// The cadence inputs let UpdateAfterPoll compute next_poll_at + velocity in
// one transaction so the post-insert count flows directly into the schedule.
type PollResult struct {
	NewETag         sql.NullString
	NewLastModified sql.NullString
	NowUnix         int64
	NewEntries      []NewEntry
	Floor           time.Duration
	Ceiling         time.Duration
	RetryAfter      time.Time     // zero -> no server floor
	CacheMaxAge     time.Duration // 0 -> no server floor
}

type Entry struct {
	ID             int64
	SubscriptionID int64
	Hash           string
	Title          string
	Author         sql.NullString
	URL            string
	Content        string
	PublishedAt    int64
	FetchedAt      int64
	Read           bool
	Saved          bool
}

type ListEntriesParams struct {
	UnreadOnly        bool
	SubscriptionID    int64 // 0 means all
	Limit             int
	CursorPublishedAt int64 // 0 means no cursor (paired with CursorID)
	CursorID          int64 // 0 means no cursor (paired with CursorPublishedAt)
}

type EntryUpdate struct {
	Read  *bool
	Saved *bool
}

// ListEntries returns entries newest-first. Bodies are NOT included to keep payloads small.
// nextPub and nextID together form the cursor for the next page (both zero if no more).
//
// Why a composite (published_at, id) cursor and not just id: backfills, re-imports,
// and clock skew can produce entries whose ID order disagrees with their published_at
// order. A bare `id < cursor` would silently drop entries whose IDs are higher than
// the cursor but whose published_at is lower. The row-value comparison `(published_at, id)
// < (?, ?)` paired with `ORDER BY published_at DESC, id DESC` is correct in all cases.
// SQLite supports row-value comparisons since 3.15.0.
func ListEntries(ctx context.Context, d *sql.DB, p ListEntriesParams) (entries []Entry, nextPub, nextID int64, err error) {
	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}

	var (
		clauses []string
		args    []any
	)
	if p.UnreadOnly {
		clauses = append(clauses, "read = 0")
	}
	if p.SubscriptionID > 0 {
		clauses = append(clauses, "subscription_id = ?")
		args = append(args, p.SubscriptionID)
	}
	if p.CursorPublishedAt > 0 {
		clauses = append(clauses, "(published_at, id) < (?, ?)")
		args = append(args, p.CursorPublishedAt, p.CursorID)
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	q := fmt.Sprintf(`
		SELECT id, subscription_id, hash, title, author, url, '' AS content,
		       published_at, fetched_at, read, saved
		FROM entries %s
		ORDER BY published_at DESC, id DESC
		LIMIT ?
	`, where)
	args = append(args, p.Limit+1)

	rows, err := d.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("list entries: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.SubscriptionID, &e.Hash, &e.Title, &e.Author,
			&e.URL, &e.Content, &e.PublishedAt, &e.FetchedAt, &e.Read, &e.Saved); err != nil {
			return nil, 0, 0, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, 0, err
	}

	if len(entries) > p.Limit {
		last := entries[p.Limit-1] // the last entry returned on this page
		nextPub = last.PublishedAt
		nextID = last.ID
		entries = entries[:p.Limit]
	}
	return entries, nextPub, nextID, nil
}

func GetEntry(ctx context.Context, d *sql.DB, id int64) (Entry, error) {
	var e Entry
	err := d.QueryRowContext(ctx, `
		SELECT id, subscription_id, hash, title, author, url, content,
		       published_at, fetched_at, read, saved
		FROM entries WHERE id = ?
	`, id).Scan(&e.ID, &e.SubscriptionID, &e.Hash, &e.Title, &e.Author,
		&e.URL, &e.Content, &e.PublishedAt, &e.FetchedAt, &e.Read, &e.Saved)
	if err != nil {
		return Entry{}, fmt.Errorf("get entry %d: %w", id, err)
	}
	return e, nil
}

func UpdateEntry(ctx context.Context, d *sql.DB, id int64, u EntryUpdate) error {
	var sets []string
	var args []any
	if u.Read != nil {
		sets = append(sets, "read = ?")
		args = append(args, boolToInt(*u.Read))
	}
	if u.Saved != nil {
		sets = append(sets, "saved = ?")
		args = append(args, boolToInt(*u.Saved))
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := d.ExecContext(ctx, fmt.Sprintf("UPDATE entries SET %s WHERE id = ?", strings.Join(sets, ", ")), args...)
	if err != nil {
		return fmt.Errorf("update entry %d: %w", id, err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpdateAfterPoll commits the success result of a poll in one transaction:
// insert entries (silently dropping duplicates on the (subscription_id, hash)
// unique constraint), recompute velocity from the post-insert state, derive
// the next-poll time via the cadence formula with server-mandated floors, and
// update the subscription row. Lives here (not subscriptions.go) so it can
// reference NewEntry.
func UpdateAfterPoll(ctx context.Context, d *sql.DB, subID int64, r PollResult) (insertedCount int, err error) {
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	for _, e := range r.NewEntries {
		res, ierr := tx.ExecContext(ctx, `
			INSERT INTO entries (subscription_id, hash, title, author, url, content, published_at, fetched_at)
			VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)
			ON CONFLICT (subscription_id, hash) DO NOTHING
		`, subID, e.Hash, e.Title, e.Author, e.URL, e.Content, e.PublishedAt, r.NowUnix)
		if ierr != nil {
			err = fmt.Errorf("insert entry: %w", ierr)
			return 0, err
		}
		n, _ := res.RowsAffected()
		insertedCount += int(n)
	}

	cutoff := r.NowUnix - int64(velocityWindow.Seconds())
	var velocity int
	if err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) * 100 / 7 FROM entries
		WHERE subscription_id = ? AND published_at >= ?
	`, subID, cutoff).Scan(&velocity); err != nil {
		err = fmt.Errorf("compute velocity: %w", err)
		return 0, err
	}

	now := time.Unix(r.NowUnix, 0).UTC()
	interval := cadence.IntervalFromVelocity(velocity, r.Floor, r.Ceiling)
	nextPoll := cadence.ApplyServerFloors(now.Add(interval), r.RetryAfter, r.CacheMaxAge, now)

	if _, err = tx.ExecContext(ctx, `
		UPDATE subscriptions
		SET etag              = ?,
		    last_modified     = ?,
		    last_poll_at      = ?,
		    next_poll_at      = ?,
		    error_count       = 0,
		    last_error        = NULL,
		    velocity_24h_x100 = ?
		WHERE id = ?
	`, r.NewETag, r.NewLastModified, r.NowUnix, nextPoll.Unix(), velocity, subID); err != nil {
		err = fmt.Errorf("update subscription: %w", err)
		return 0, err
	}

	if err = tx.Commit(); err != nil {
		err = fmt.Errorf("commit: %w", err)
		return 0, err
	}
	return insertedCount, nil
}
