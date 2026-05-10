ALTER TABLE subscriptions ADD COLUMN extract           INTEGER NOT NULL DEFAULT 0;
ALTER TABLE subscriptions ADD COLUMN extract_selector  TEXT    NOT NULL DEFAULT '';
ALTER TABLE entries       ADD COLUMN extract_failed    INTEGER NOT NULL DEFAULT 0;
