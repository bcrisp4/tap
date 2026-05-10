CREATE VIRTUAL TABLE entries_fts USING fts5(
    title,
    content_text,
    author,
    content='',
    tokenize='unicode61'
);

CREATE TRIGGER entries_fts_insert AFTER INSERT ON entries BEGIN
    INSERT INTO entries_fts(rowid, title, content_text, author)
    VALUES (new.id, new.title, tap_strip_html(new.content), new.author);
END;

CREATE TRIGGER entries_fts_delete AFTER DELETE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid) VALUES ('delete', old.id);
END;

CREATE TRIGGER entries_fts_update AFTER UPDATE ON entries BEGIN
    INSERT INTO entries_fts(entries_fts, rowid) VALUES ('delete', old.id);
    INSERT INTO entries_fts(rowid, title, content_text, author)
    VALUES (new.id, new.title, tap_strip_html(new.content), new.author);
END;

-- Back-fill existing entries into FTS index.
INSERT INTO entries_fts(rowid, title, content_text, author)
SELECT id, title, tap_strip_html(content), author FROM entries;
