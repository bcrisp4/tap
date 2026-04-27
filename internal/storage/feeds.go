package storage

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Feed mirrors the feeds table. Optional columns use *T for
// nullability; INTEGER booleans expose as bool.
type Feed struct {
	ID          int64
	UserID      int64
	CategoryID  *int64
	IconID      *int64
	Title       string
	FeedURL     string
	SiteURL     *string
	Description *string

	ETag         *string
	LastModified *string
	LastPolledAt *int64
	NextPollAt   *int64
	PollInterval int64
	ErrorCount   int
	LastError    *string

	WeeklyEntryCount int

	Crawler      bool
	ScraperRules *string

	Disabled           bool
	IgnoreEntryUpdates bool

	UserAgent            *string
	Cookie               *string
	Username             *string
	Password             *string
	ProxyURL             *string
	DisableHTTP2         bool
	AllowSelfSignedCerts bool

	CreatedAt int64
	UpdatedAt int64
}

const feedSelectCols = `id, user_id, category_id, icon_id, title, feed_url, site_url, description,
		etag, last_modified, last_polled_at, next_poll_at, poll_interval, error_count, last_error,
		weekly_entry_count, crawler, scraper_rules, disabled, ignore_entry_updates,
		user_agent, cookie, username, password, proxy_url, disable_http2, allow_self_signed_certs,
		created_at, updated_at`

func scanFeed(row interface{ Scan(...any) error }) (*Feed, error) {
	f := &Feed{}
	var crawler, disabled, ignoreUpd, dh2, ssc int
	err := row.Scan(
		&f.ID, &f.UserID, &f.CategoryID, &f.IconID, &f.Title, &f.FeedURL, &f.SiteURL, &f.Description,
		&f.ETag, &f.LastModified, &f.LastPolledAt, &f.NextPollAt, &f.PollInterval, &f.ErrorCount, &f.LastError,
		&f.WeeklyEntryCount, &crawler, &f.ScraperRules, &disabled, &ignoreUpd,
		&f.UserAgent, &f.Cookie, &f.Username, &f.Password, &f.ProxyURL, &dh2, &ssc,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	f.Crawler = crawler == 1
	f.Disabled = disabled == 1
	f.IgnoreEntryUpdates = ignoreUpd == 1
	f.DisableHTTP2 = dh2 == 1
	f.AllowSelfSignedCerts = ssc == 1
	return f, nil
}

func (s *Store) GetFeed(ctx context.Context, userID, id int64) (*Feed, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+feedSelectCols+` FROM feeds WHERE id = ? AND user_id = ?`, id, userID)
	f, err := scanFeed(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return f, err
}

func (s *Store) ListFeeds(ctx context.Context, userID int64) ([]*Feed, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+feedSelectCols+` FROM feeds WHERE user_id = ? ORDER BY title COLLATE NOCASE`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Feed
	for rows.Next() {
		f, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) CreateFeed(ctx context.Context, f *Feed) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO feeds(user_id, category_id, icon_id, title, feed_url, site_url, description,
			etag, last_modified, last_polled_at, next_poll_at, poll_interval, error_count, last_error,
			weekly_entry_count, crawler, scraper_rules, disabled, ignore_entry_updates,
			user_agent, cookie, username, password, proxy_url, disable_http2, allow_self_signed_certs)
		 VALUES(?, ?, ?, ?, ?, ?, ?,
		        ?, ?, ?, ?, ?, ?, ?,
		        ?, ?, ?, ?, ?,
		        ?, ?, ?, ?, ?, ?, ?)`,
		f.UserID, f.CategoryID, f.IconID, f.Title, f.FeedURL, f.SiteURL, f.Description,
		f.ETag, f.LastModified, f.LastPolledAt, f.NextPollAt, f.PollInterval, f.ErrorCount, f.LastError,
		f.WeeklyEntryCount, boolInt(f.Crawler), f.ScraperRules, boolInt(f.Disabled), boolInt(f.IgnoreEntryUpdates),
		f.UserAgent, f.Cookie, f.Username, f.Password, f.ProxyURL, boolInt(f.DisableHTTP2), boolInt(f.AllowSelfSignedCerts),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateFeed(ctx context.Context, f *Feed) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE feeds SET
			category_id = ?, icon_id = ?, title = ?, feed_url = ?, site_url = ?, description = ?,
			etag = ?, last_modified = ?, last_polled_at = ?, next_poll_at = ?, poll_interval = ?, error_count = ?, last_error = ?,
			weekly_entry_count = ?, crawler = ?, scraper_rules = ?, disabled = ?, ignore_entry_updates = ?,
			user_agent = ?, cookie = ?, username = ?, password = ?, proxy_url = ?, disable_http2 = ?, allow_self_signed_certs = ?,
			updated_at = unixepoch()
		 WHERE id = ? AND user_id = ?`,
		f.CategoryID, f.IconID, f.Title, f.FeedURL, f.SiteURL, f.Description,
		f.ETag, f.LastModified, f.LastPolledAt, f.NextPollAt, f.PollInterval, f.ErrorCount, f.LastError,
		f.WeeklyEntryCount, boolInt(f.Crawler), f.ScraperRules, boolInt(f.Disabled), boolInt(f.IgnoreEntryUpdates),
		f.UserAgent, f.Cookie, f.Username, f.Password, f.ProxyURL, boolInt(f.DisableHTTP2), boolInt(f.AllowSelfSignedCerts),
		f.ID, f.UserID,
	)
	if err != nil {
		return err
	}
	return rowsOrNotFound(res)
}

func (s *Store) DeleteFeed(ctx context.Context, userID, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM feeds WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	return rowsOrNotFound(res)
}

// ListDueFeeds returns ids of feeds whose next_poll_at <= now, that
// are not disabled, that have fewer than 10 consecutive errors, and
// that are not in the exclude set. Used by the poller dispatcher.
func (s *Store) ListDueFeeds(ctx context.Context, now int64, limit int, exclude []int64) ([]int64, error) {
	q := `SELECT id FROM feeds
	      WHERE next_poll_at <= ?
	        AND disabled = 0
	        AND error_count < 10`
	args := []any{now}
	if len(exclude) > 0 {
		placeholders := strings.Repeat("?,", len(exclude))
		placeholders = placeholders[:len(placeholders)-1]
		q += " AND id NOT IN (" + placeholders + ")"
		for _, id := range exclude {
			args = append(args, id)
		}
	}
	q += " ORDER BY next_poll_at ASC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CommitPollSuccess writes the new entries and updates the feed in a
// single IMMEDIATE transaction. Duplicate (feed_id, hash) collisions
// are silently dropped (race tolerance). Returns the recomputed
// weekly_entry_count.
func (s *Store) CommitPollSuccess(
	ctx context.Context,
	feedID int64,
	entries []*Entry,
	etag, lastModified string,
	errCount int,
	nextPollAt int64,
) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, e := range entries {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO entries(feed_id, user_id, hash, title, url, comments_url,
				author, summary, content, published_at, reading_time, extraction_failed)
			 VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.FeedID, e.UserID, e.Hash, e.Title, e.URL, e.CommentsURL,
			e.Author, e.Summary, e.Content, e.PublishedAt, e.ReadingTime,
			boolInt(e.ExtractionFailed),
		)
		if err != nil && !isUniqueConstraint(err) {
			return 0, err
		}
	}

	var weekly int
	if err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM entries
		 WHERE feed_id = ? AND created_at > unixepoch() - 7*86400`, feedID).
		Scan(&weekly); err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE feeds SET
			etag = NULLIF(?, ''),
			last_modified = NULLIF(?, ''),
			last_polled_at = unixepoch(),
			next_poll_at = ?,
			error_count = ?,
			last_error = NULL,
			weekly_entry_count = ?,
			updated_at = unixepoch()
		 WHERE id = ?`,
		etag, lastModified, nextPollAt, errCount, weekly, feedID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return weekly, nil
}

// CommitPollNotModified handles the 304 case: feed timestamps + reset
// error_count, no entry inserts.
func (s *Store) CommitPollNotModified(
	ctx context.Context,
	feedID int64,
	etag, lastModified string,
	nextPollAt int64,
) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE feeds SET
			etag = COALESCE(NULLIF(?, ''), etag),
			last_modified = COALESCE(NULLIF(?, ''), last_modified),
			last_polled_at = unixepoch(),
			next_poll_at = ?,
			error_count = 0,
			last_error = NULL,
			updated_at = unixepoch()
		 WHERE id = ?`,
		etag, lastModified, nextPollAt, feedID)
	return err
}

// CommitPollFailure records the failure: increment error_count, store
// last_error, push next_poll_at out per the adaptive backoff (the
// caller computes the timestamp).
func (s *Store) CommitPollFailure(
	ctx context.Context,
	feedID int64,
	errCount int,
	lastError string,
	nextPollAt int64,
) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE feeds SET
			error_count = ?,
			last_error = NULLIF(?, ''),
			last_polled_at = unixepoch(),
			next_poll_at = ?,
			updated_at = unixepoch()
		 WHERE id = ?`,
		errCount, lastError, nextPollAt, feedID)
	return err
}

// isUniqueConstraint reports whether err is a SQLite UNIQUE
// constraint violation. Cross-driver: matches both modernc and mattn.
func isUniqueConstraint(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique")
}
