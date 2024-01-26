CREATE TABLE IF NOT EXISTS sync (
    id INTEGER PRIMARY KEY,
    syncing BOOLEAN,
    start_time INTEGER,
    end_time INTEGER,
    total_files INTEGER,
    synced_files INTEGER,
    sync_error TEXT
);

INSERT INTO sync VALUES(1, false, 0, 0, 0, 0, '');