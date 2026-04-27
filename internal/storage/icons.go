package storage

import (
	"context"
	"database/sql"
	"errors"
)

// Icon mirrors the icons table.
type Icon struct {
	ID       int64
	Hash     string
	MIMEType string
	Content  []byte
}

func (s *Store) GetIconByHash(ctx context.Context, hash string) (*Icon, error) {
	i := &Icon{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, hash, mime_type, content FROM icons WHERE hash = ?`, hash).
		Scan(&i.ID, &i.Hash, &i.MIMEType, &i.Content)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return i, nil
}

func (s *Store) InsertIcon(ctx context.Context, hash, mime string, content []byte) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO icons(hash, mime_type, content) VALUES(?, ?, ?)`,
		hash, mime, content)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
