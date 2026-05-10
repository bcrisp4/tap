package db

import (
	"context"
	"database/sql"
	"fmt"
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
// Returns up to limit results ordered by BM25 rank (most relevant first).
// No cursor pagination — the spec caps M9 search at 50 results.
func SearchEntries(ctx context.Context, d *sql.DB, userID int64, query string, limit int) ([]SearchResult, error) {
	rows, err := d.QueryContext(ctx, `
		SELECT e.id, e.subscription_id, e.title, e.url, COALESCE(e.author, ''),
		       e.published_at, e.read, e.saved,
		       bm25(entries_fts) AS rank
		FROM   entries_fts
		JOIN   entries       e  ON e.id = entries_fts.rowid
		JOIN   subscriptions s  ON s.id = e.subscription_id
		WHERE  entries_fts MATCH ?
		  AND  s.user_id = ?
		ORDER  BY rank
		LIMIT  ?
	`, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("search entries: %w", err)
	}
	defer rows.Close()

	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		var readInt, savedInt int
		if err := rows.Scan(&r.ID, &r.SubscriptionID, &r.Title, &r.URL, &r.Author,
			&r.PublishedAt, &readInt, &savedInt, &r.Rank); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		r.Read = readInt != 0
		r.Saved = savedInt != 0
		out = append(out, r)
	}
	return out, rows.Err()
}
