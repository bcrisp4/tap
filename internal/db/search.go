package db

import (
	"context"
	"database/sql"
	"fmt"
	"math"
)

type SearchResult struct {
	ID             int64
	SubscriptionID int64
	Title          string
	URL            string
	Author         string
	PublishedAt    int64
	Read           bool
	Saved          bool
	Rank           float64
}

// SearchEntries performs an FTS5 full-text search across the caller's entries.
// Returns matching results ordered by BM25 rank (most relevant first) then by
// entry ID descending. cursor is an exclusive upper bound on entry ID for
// keyset pagination; pass math.MaxInt64 for the first page.
func SearchEntries(ctx context.Context, d *sql.DB, userID int64, query string, limit int, cursor int64) ([]SearchResult, int64, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT e.id, e.subscription_id, e.title, e.url, e.author,
		       e.published_at, e.read, e.saved,
		       bm25(entries_fts) AS rank
		FROM   entries_fts
		JOIN   entries       e  ON e.id = entries_fts.rowid
		JOIN   subscriptions s  ON s.id = e.subscription_id
		WHERE  entries_fts MATCH ?
		  AND  s.user_id = ?
		  AND  e.id < ?
		ORDER  BY rank, e.id DESC
		LIMIT  ?
	`, query, userID, cursor, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("search entries: %w", err)
	}
	defer rows.Close()

	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		var readInt, savedInt int
		if err := rows.Scan(&r.ID, &r.SubscriptionID, &r.Title, &r.URL, &r.Author,
			&r.PublishedAt, &readInt, &savedInt, &r.Rank); err != nil {
			return nil, 0, fmt.Errorf("scan search result: %w", err)
		}
		r.Read = readInt != 0
		r.Saved = savedInt != 0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var nextCursor int64
	if len(out) == limit {
		nextCursor = out[len(out)-1].ID
	} else {
		nextCursor = math.MaxInt64
	}
	return out, nextCursor, nil
}
