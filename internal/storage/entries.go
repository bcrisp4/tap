package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Entry mirrors the entries table.
type Entry struct {
	ID               int64
	FeedID           int64
	UserID           int64
	Hash             string
	Title            string
	URL              *string
	CommentsURL      *string
	Author           *string
	Summary          *string
	Content          *string
	PublishedAt      *int64
	ReadingTime      int
	Read             bool
	ReadAt           *int64
	Saved            bool
	SavedAt          *int64
	ExtractionFailed bool
	CreatedAt        int64
	ChangedAt        int64
}

// EntriesFilter mirrors the §6 list query params (subset for Plan 02).
type EntriesFilter struct {
	Status     string // "" (all), "unread", "read"
	Saved      *bool
	FeedID     *int64
	CategoryID *int64
	Sort       string // "published_at" (default), "created_at"
	Order      string // "desc" (default), "asc"
	Limit      int
	Offset     int
}

// BulkScope narrows the rows BulkMarkRead acts on. Each non-nil field
// adds an AND-filter; both nil means every unread row for the user.
// In practice callers set at most one (the API exposes "mark feed
// read" and "mark category read" as separate verbs), but the
// intersection is well-defined when both are supplied.
type BulkScope struct {
	FeedID     *int64
	CategoryID *int64
}

const entrySelectCols = `id, feed_id, user_id, hash, title, url, comments_url, author, summary, content,
		published_at, reading_time, read, read_at, saved, saved_at, extraction_failed, created_at, changed_at`

func scanEntry(row interface{ Scan(...any) error }) (*Entry, error) {
	e := &Entry{}
	var read, saved, extf int
	err := row.Scan(
		&e.ID, &e.FeedID, &e.UserID, &e.Hash, &e.Title, &e.URL, &e.CommentsURL, &e.Author, &e.Summary, &e.Content,
		&e.PublishedAt, &e.ReadingTime, &read, &e.ReadAt, &saved, &e.SavedAt, &extf, &e.CreatedAt, &e.ChangedAt,
	)
	if err != nil {
		return nil, err
	}
	e.Read = read == 1
	e.Saved = saved == 1
	e.ExtractionFailed = extf == 1
	return e, nil
}

func (s *Store) GetEntry(ctx context.Context, userID, id int64) (*Entry, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+entrySelectCols+` FROM entries WHERE id = ? AND user_id = ?`, id, userID)
	e, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return e, err
}

func (s *Store) InsertEntry(ctx context.Context, e *Entry) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO entries(feed_id, user_id, hash, title, url, comments_url, author, summary, content,
			published_at, reading_time, read, saved, extraction_failed)
		 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?,
		        ?, ?, ?, ?, ?)`,
		e.FeedID, e.UserID, e.Hash, e.Title, e.URL, e.CommentsURL, e.Author, e.Summary, e.Content,
		e.PublishedAt, e.ReadingTime, boolInt(e.Read), boolInt(e.Saved), boolInt(e.ExtractionFailed),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListEntries(ctx context.Context, userID int64, f EntriesFilter) ([]*Entry, error) {
	q := `SELECT ` + entrySelectCols + ` FROM entries WHERE user_id = ?`
	args := []any{userID}

	switch f.Status {
	case "unread":
		q += " AND read = 0"
	case "read":
		q += " AND read = 1"
	}
	if f.Saved != nil {
		q += " AND saved = ?"
		args = append(args, boolInt(*f.Saved))
	}
	if f.FeedID != nil {
		q += " AND feed_id = ?"
		args = append(args, *f.FeedID)
	}
	if f.CategoryID != nil {
		q += " AND feed_id IN (SELECT id FROM feeds WHERE category_id = ?)"
		args = append(args, *f.CategoryID)
	}

	sort := "published_at"
	if f.Sort == "created_at" {
		sort = "created_at"
	}
	order := "DESC"
	if strings.EqualFold(f.Order, "asc") {
		order = "ASC"
	}
	q += " ORDER BY " + sort + " " + order

	if f.Limit <= 0 {
		f.Limit = 50
	}
	q += " LIMIT ? OFFSET ?"
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateEntryState toggles read and/or saved. Nil leaves the field
// alone. Sets read_at / saved_at to unixepoch() only on the 0->1
// transition (preserving the original timestamp on idempotent calls);
// clears them on transition to true->false.
func (s *Store) UpdateEntryState(ctx context.Context, userID, id int64, read, saved *bool) error {
	if read == nil && saved == nil {
		return nil
	}
	parts := []string{}
	args := []any{}
	if read != nil {
		parts = append(parts, "read = ?")
		args = append(args, boolInt(*read))
		if *read {
			// Preserve existing read_at on idempotent re-mark.
			parts = append(parts, "read_at = CASE WHEN read = 0 THEN unixepoch() ELSE read_at END")
		} else {
			parts = append(parts, "read_at = NULL")
		}
	}
	if saved != nil {
		parts = append(parts, "saved = ?")
		args = append(args, boolInt(*saved))
		if *saved {
			parts = append(parts, "saved_at = CASE WHEN saved = 0 THEN unixepoch() ELSE saved_at END")
		} else {
			parts = append(parts, "saved_at = NULL")
		}
	}
	parts = append(parts, "changed_at = unixepoch()")
	args = append(args, id, userID)

	res, err := s.db.ExecContext(ctx,
		`UPDATE entries SET `+strings.Join(parts, ", ")+` WHERE id = ? AND user_id = ?`, args...)
	if err != nil {
		return err
	}
	return rowsOrNotFound(res)
}

// EntryExists reports whether an entry with (feed_id, hash) is in
// the entries table. Used by the poller's dedup probe before insert.
func (s *Store) EntryExists(ctx context.Context, feedID int64, hash string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM entries WHERE feed_id = ? AND hash = ?`,
		feedID, hash).Scan(&n)
	return n > 0, err
}

// BulkMarkRead marks every matching entry read. Idempotent.
func (s *Store) BulkMarkRead(ctx context.Context, userID int64, scope BulkScope) error {
	q := `UPDATE entries SET read = 1, read_at = unixepoch(), changed_at = unixepoch()
	      WHERE user_id = ? AND read = 0`
	args := []any{userID}
	if scope.FeedID != nil {
		q += " AND feed_id = ?"
		args = append(args, *scope.FeedID)
	}
	if scope.CategoryID != nil {
		q += " AND feed_id IN (SELECT id FROM feeds WHERE category_id = ?)"
		args = append(args, *scope.CategoryID)
	}
	_, err := s.db.ExecContext(ctx, q, args...)
	return err
}
