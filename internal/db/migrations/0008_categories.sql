CREATE TABLE categories (
    id         INTEGER PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    created_at INTEGER NOT NULL,
    UNIQUE (user_id, name)
);
CREATE INDEX idx_categories_user ON categories(user_id);

ALTER TABLE subscriptions ADD COLUMN category_id INTEGER REFERENCES categories(id) ON DELETE SET NULL;
CREATE INDEX idx_subscriptions_category ON subscriptions(category_id);
