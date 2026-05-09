package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies any pending SQL migrations in lexical order. Idempotent.
func Migrate(ctx context.Context, d *sql.DB) error {
	if _, err := d.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	var applied int
	if err := d.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&applied); err != nil {
		return fmt.Errorf("query applied version: %w", err)
	}

	for _, name := range files {
		v, err := versionFromFilename(name)
		if err != nil {
			return err
		}
		if v <= applied {
			continue
		}
		body, err := migrationsFS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		tx, err := d.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", v, time.Now().Unix()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit %s: %w", name, err)
		}
	}
	return nil
}

func versionFromFilename(name string) (int, error) {
	base := strings.TrimPrefix(name, "migrations/")
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return 0, fmt.Errorf("migration %q: cannot parse version prefix", name)
	}
	v, err := strconv.Atoi(base[:idx])
	if err != nil {
		return 0, fmt.Errorf("migration %q: version not numeric: %w", name, err)
	}
	return v, nil
}
