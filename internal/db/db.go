// Package db opens a SQLite connection with Tap's required pragmas
// and provides the migration runner.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Open opens (or creates) a SQLite database at path and applies Tap's
// pragmas: foreign_keys = ON, journal_mode = WAL. Returns the opened
// *sql.DB, which the caller must Close.
func Open(path string) (*sql.DB, error) {
	// modernc registers as "sqlite". Use a DSN that enables a few
	// useful query parameters; they're applied at connection time.
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// Verify by pinging — sql.Open is lazy.
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, fmt.Errorf("ping sqlite at %s: %w", path, err)
	}
	// Defensive: re-issue the pragmas in case the DSN syntax differs
	// across modernc versions.
	for _, p := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
	} {
		if _, err := d.Exec(p); err != nil {
			d.Close()
			return nil, fmt.Errorf("apply pragma %q: %w", p, err)
		}
	}
	// SQLite is single-writer; one connection is the safe upper bound.
	d.SetMaxOpenConns(1)
	return d, nil
}
