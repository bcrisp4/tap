-- sessions.user_id must become nullable for anonymous WebAuthn challenge sessions.
-- SQLite cannot drop NOT NULL via ALTER TABLE; recreate using rename→create→copy→drop.
ALTER TABLE sessions RENAME TO sessions_old;

CREATE TABLE sessions (
    id                  INTEGER PRIMARY KEY,
    user_id             INTEGER REFERENCES users(id) ON DELETE CASCADE,
    token_hash          TEXT    NOT NULL UNIQUE,
    csrf_token          TEXT    NOT NULL,
    created_at          INTEGER NOT NULL,
    last_seen_at        INTEGER NOT NULL,
    idle_expires_at     INTEGER NOT NULL,
    absolute_expires_at INTEGER NOT NULL,
    user_agent          TEXT    NOT NULL DEFAULT '',
    address             TEXT    NOT NULL DEFAULT '',
    webauthn_challenge  BLOB
);
-- The original idx_sessions_user from 0005 still exists on sessions_old;
-- drop it so the new index can use the same name.
DROP INDEX IF EXISTS idx_sessions_user;
CREATE INDEX idx_sessions_user ON sessions(user_id);

INSERT INTO sessions (id, user_id, token_hash, csrf_token, created_at, last_seen_at,
                      idle_expires_at, absolute_expires_at, user_agent, address, webauthn_challenge)
SELECT                id, user_id, token_hash, csrf_token, created_at, last_seen_at,
                      idle_expires_at, absolute_expires_at, '',         '',      NULL
FROM sessions_old;

DROP TABLE sessions_old;

-- The three new columns are included in the table recreation above.

CREATE TABLE pending_logins (
    id          INTEGER PRIMARY KEY,
    token_hash  TEXT    NOT NULL UNIQUE,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  INTEGER NOT NULL,
    expires_at  INTEGER NOT NULL
);
CREATE INDEX idx_pending_logins_expires ON pending_logins(expires_at);

CREATE TABLE totp_secrets (
    id                INTEGER PRIMARY KEY,
    user_id           INTEGER NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    secret_encrypted  BLOB    NOT NULL,
    confirmed         INTEGER NOT NULL DEFAULT 0,
    created_at        INTEGER NOT NULL
);

CREATE TABLE recovery_codes (
    id           INTEGER PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash    TEXT    NOT NULL,
    consumed_at  INTEGER
);
CREATE INDEX idx_recovery_codes_user ON recovery_codes(user_id);

CREATE TABLE passkeys (
    id            INTEGER PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    credential_id BLOB    NOT NULL UNIQUE,
    public_key    BLOB    NOT NULL,
    sign_counter  INTEGER NOT NULL DEFAULT 0,
    aaguid        BLOB,
    label         TEXT    NOT NULL DEFAULT '',
    created_at    INTEGER NOT NULL
);
CREATE INDEX idx_passkeys_user ON passkeys(user_id);
