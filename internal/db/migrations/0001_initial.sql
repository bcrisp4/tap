CREATE TABLE subscriptions (
    id            INTEGER PRIMARY KEY,
    title         TEXT NOT NULL,
    feed_url      TEXT NOT NULL UNIQUE,
    site_url      TEXT,
    last_poll_at  INTEGER,
    next_poll_at  INTEGER NOT NULL,
    etag          TEXT,
    last_modified TEXT,
    error_count   INTEGER NOT NULL DEFAULT 0,
    last_error    TEXT,
    created_at    INTEGER NOT NULL
);
CREATE INDEX idx_subscriptions_next_poll ON subscriptions(next_poll_at);

CREATE TABLE entries (
    id              INTEGER PRIMARY KEY,
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    hash            TEXT NOT NULL,
    title           TEXT NOT NULL,
    author          TEXT,
    url             TEXT NOT NULL,
    content         TEXT NOT NULL,
    published_at    INTEGER NOT NULL,
    fetched_at      INTEGER NOT NULL,
    read            INTEGER NOT NULL DEFAULT 0,
    saved           INTEGER NOT NULL DEFAULT 0,
    UNIQUE (subscription_id, hash)
);
CREATE INDEX idx_entries_published    ON entries(published_at DESC);
CREATE INDEX idx_entries_subscription ON entries(subscription_id, published_at DESC);
CREATE INDEX idx_entries_unread       ON entries(read, published_at DESC);
