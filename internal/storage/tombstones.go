package storage

import "context"

func (s *Store) InsertTombstone(ctx context.Context, feedID int64, hash string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO entry_tombstones(feed_id, hash) VALUES(?, ?)`, feedID, hash)
	return err
}

func (s *Store) HasTombstone(ctx context.Context, feedID int64, hash string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM entry_tombstones WHERE feed_id = ? AND hash = ?`,
		feedID, hash).Scan(&n)
	return n > 0, err
}
