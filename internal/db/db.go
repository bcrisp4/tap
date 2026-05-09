// Package db owns SQLite connection setup, migrations, and per-table query helpers.
package db

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver
)

// Open opens (or creates) a SQLite database at path and applies the PRAGMAs
// every Tap process expects. Pass ":memory:" for an in-memory DB.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)

	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	// SQLite in-memory databases are per-connection. If the pool opens multiple
	// connections, each gets an independent (empty) database. Pin to one
	// connection so migrations and data are visible across all callers.
	if path == ":memory:" {
		d.SetMaxOpenConns(1)
	}

	if err := d.PingContext(ctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return d, nil
}
