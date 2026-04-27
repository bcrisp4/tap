-- 0001_baseline.sql
-- Bootstrap: create the schema_version tracking table.
-- (CREATE TABLE IF NOT EXISTS lets Migrate() create this idempotently
-- inline before any migration runs, so the baseline file is itself
-- safely re-runnable.)
CREATE TABLE IF NOT EXISTS schema_version (
    version    TEXT PRIMARY KEY,
    applied_at INTEGER NOT NULL DEFAULT (unixepoch())
);
