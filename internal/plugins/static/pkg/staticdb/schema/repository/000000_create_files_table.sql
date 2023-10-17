CREATE TABLE IF NOT EXISTS files (
    tag TEXT PRIMARY KEY,
    id TEXT,
    name TEXT,
    upload_time INTEGER,
    size INTEGER
);

CREATE INDEX fileid_idx ON files(id);

CREATE TABLE IF NOT EXISTS file_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag TEXT,
    name TEXT,
    file_id TEXT,
    new_file_id TEXT,
    created_at INTEGER,
    updated BOOLEAN,
    deleted BOOLEAN
);

CREATE TRIGGER IF NOT EXISTS file_update_trigger
    AFTER UPDATE ON files
    WHEN old.id <> new.id
BEGIN
    INSERT INTO file_history (
        tag,
        name,
        file_id,
        new_file_id,
        created_at,
        updated
)
VALUES
    (
        old.tag,
        old.name,
        old.id,
        new.id,
        UNIXEPOCH(),
        true
    );
END;

CREATE TRIGGER IF NOT EXISTS file_delete_trigger
    AFTER DELETE ON files
BEGIN
    INSERT INTO file_history (
        tag,
        name,
        file_id,
        created_at,
        deleted
)
VALUES
    (
        old.tag,
        old.name,
        old.id,
        UNIXEPOCH(),
        true
    );
END;