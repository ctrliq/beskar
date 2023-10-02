CREATE TABLE IF NOT EXISTS reposync (
    id INTEGER PRIMARY KEY,
    syncing BOOLEAN,
    last_sync_time INTEGER,
    total_packages INTEGER,
    synced_packages INTEGER
);

INSERT INTO reposync VALUES(1, false, 0, 0, 0);