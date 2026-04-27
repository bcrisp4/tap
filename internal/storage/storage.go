// Package storage holds the SQLite-backed repositories for every
// domain entity in Tap. One Store instance wraps a *sql.DB; per-domain
// methods live in sibling files (users.go, feeds.go, …).
package storage

import (
	"database/sql"
	"errors"
)

// ErrNotFound is the sentinel returned by Get* methods when the row
// does not exist. Wrap it with errors.Is at call sites.
var ErrNotFound = errors.New("storage: not found")

// ErrBadQuery signals that a user-supplied query (e.g. an FTS5 MATCH
// expression) failed to parse. Callers map it to HTTP 400.
var ErrBadQuery = errors.New("storage: bad query")

// Store is the entrypoint for all repository operations.
type Store struct {
	db *sql.DB
}

// New wraps an open *sql.DB.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB exposes the underlying *sql.DB. Callers that need a transaction
// (the poller, OPML import) use this directly.
func (s *Store) DB() *sql.DB {
	return s.db
}
