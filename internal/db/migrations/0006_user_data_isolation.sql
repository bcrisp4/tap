-- Recreate subscriptions with user_id and UNIQUE(user_id, feed_url).
-- The global UNIQUE(feed_url) from M1 would prevent two users from
-- subscribing to the same feed URL.
--
-- We must also recreate entries because SQLite updates FK references in
-- dependent tables when we rename subscriptions → subscriptions_old,
-- breaking entries.subscription_id. The rename→create→copy→drop pattern
-- is applied to both tables.
ALTER TABLE subscriptions RENAME TO subscriptions_old;
ALTER TABLE entries RENAME TO entries_old;

CREATE TABLE subscriptions (
    id                INTEGER PRIMARY KEY,
    user_id           INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title             TEXT    NOT NULL,
    feed_url          TEXT    NOT NULL,
    site_url          TEXT,
    last_poll_at      INTEGER,
    next_poll_at      INTEGER NOT NULL,
    etag              TEXT,
    last_modified     TEXT,
    error_count       INTEGER NOT NULL DEFAULT 0,
    last_error        TEXT,
    created_at        INTEGER NOT NULL,
    velocity_24h_x100 INTEGER NOT NULL DEFAULT 0,
    extract           INTEGER NOT NULL DEFAULT 0,
    extract_selector  TEXT    NOT NULL DEFAULT '',
    cookie            TEXT    NOT NULL DEFAULT '',
    basic_auth_user   TEXT    NOT NULL DEFAULT '',
    basic_auth_pass   TEXT    NOT NULL DEFAULT '',
    UNIQUE (user_id, feed_url)
);
-- The original idx_subscriptions_next_poll from 0001 still exists on
-- subscriptions_old; drop it so the new index can use the same name.
DROP INDEX IF EXISTS idx_subscriptions_next_poll;
CREATE INDEX idx_subscriptions_next_poll ON subscriptions(next_poll_at);
CREATE INDEX idx_subscriptions_user      ON subscriptions(user_id);

-- Fresh install: subscriptions_old is empty, so this copies nothing.
INSERT INTO subscriptions
    SELECT id, 0, title, feed_url, site_url, last_poll_at, next_poll_at,
           etag, last_modified, error_count, last_error, created_at,
           velocity_24h_x100, extract, extract_selector, cookie,
           basic_auth_user, basic_auth_pass
    FROM subscriptions_old;

CREATE TABLE entries (
    id              INTEGER PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    hash            TEXT    NOT NULL,
    title           TEXT    NOT NULL,
    author          TEXT,
    url             TEXT    NOT NULL,
    content         TEXT    NOT NULL,
    published_at    INTEGER NOT NULL,
    fetched_at      INTEGER NOT NULL,
    read            INTEGER NOT NULL DEFAULT 0,
    saved           INTEGER NOT NULL DEFAULT 0,
    extract_failed  INTEGER NOT NULL DEFAULT 0,
    UNIQUE (subscription_id, hash)
);
CREATE INDEX idx_entries_user ON entries(user_id, published_at DESC);

-- Copy existing entries (empty on fresh install).
INSERT INTO entries
    SELECT id, 0, subscription_id, hash, title, author, url, content,
           published_at, fetched_at, read, saved, extract_failed
    FROM entries_old;

DROP TABLE entries_old;
DROP TABLE subscriptions_old;
