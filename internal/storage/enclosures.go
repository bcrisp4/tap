package storage

import "context"

// Enclosure mirrors the enclosures table.
type Enclosure struct {
	ID       int64
	EntryID  int64
	URL      string
	MIMEType string
	Size     int64
}

func (s *Store) InsertEnclosure(ctx context.Context, entryID int64, url, mime string, size int64) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO enclosures(entry_id, url, mime_type, size) VALUES(?, ?, ?, ?)`,
		entryID, url, mime, size)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListEnclosures(ctx context.Context, entryID int64) ([]*Enclosure, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, entry_id, url, mime_type, size FROM enclosures WHERE entry_id = ? ORDER BY id`, entryID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Enclosure
	for rows.Next() {
		e := &Enclosure{}
		if err := rows.Scan(&e.ID, &e.EntryID, &e.URL, &e.MIMEType, &e.Size); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
