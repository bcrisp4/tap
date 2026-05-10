CREATE TABLE users (
    id            INTEGER PRIMARY KEY,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL CHECK (role IN ('admin','user')),
    created_at    INTEGER NOT NULL,
    disabled_at   INTEGER
);

CREATE TABLE sessions (
    id                  INTEGER PRIMARY KEY,
    user_id             INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash          TEXT    NOT NULL UNIQUE,
    csrf_token          TEXT    NOT NULL,
    created_at          INTEGER NOT NULL,
    last_seen_at        INTEGER NOT NULL,
    idle_expires_at     INTEGER NOT NULL,
    absolute_expires_at INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

ALTER TABLE subscriptions ADD COLUMN cookie          TEXT NOT NULL DEFAULT '';
ALTER TABLE subscriptions ADD COLUMN basic_auth_user TEXT NOT NULL DEFAULT '';
ALTER TABLE subscriptions ADD COLUMN basic_auth_pass TEXT NOT NULL DEFAULT '';
