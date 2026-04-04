CREATE VIRTUAL TABLE title_names_fts USING fts5(
    name,
    content='title_names',
    content_rowid='id',
    tokenize='unicode61 remove_diacritics 2'
);

INSERT INTO title_names_fts(rowid, name) SELECT id, name FROM title_names;

CREATE TRIGGER title_names_fts_ai AFTER INSERT ON title_names BEGIN
    INSERT INTO title_names_fts(rowid, name) VALUES (new.id, new.name);
END;

CREATE TRIGGER title_names_fts_ad AFTER DELETE ON title_names BEGIN
    INSERT INTO title_names_fts(title_names_fts, rowid, name) VALUES('delete', old.id, old.name);
END;

CREATE TRIGGER title_names_fts_au AFTER UPDATE ON title_names BEGIN
    INSERT INTO title_names_fts(title_names_fts, rowid, name) VALUES('delete', old.id, old.name);
    INSERT INTO title_names_fts(rowid, name) VALUES (new.id, new.name);
END;
