CREATE TABLE tombstones (
    subscription_id INTEGER NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    entry_hash      TEXT    NOT NULL,
    deleted_at      INTEGER NOT NULL,
    PRIMARY KEY (subscription_id, entry_hash)
);
CREATE INDEX idx_tombstones_subscription ON tombstones(subscription_id);
