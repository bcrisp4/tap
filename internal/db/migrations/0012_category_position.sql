ALTER TABLE categories ADD COLUMN position INTEGER NOT NULL DEFAULT 0;
CREATE INDEX idx_categories_user_position ON categories(user_id, position);
