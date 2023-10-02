CREATE TABLE IF NOT EXISTS packages (
    name TEXT PRIMARY KEY,
    id TEXT,
    meta_primary BLOB,
    meta_filelists BLOB,
    meta_other BLOB
);

CREATE INDEX pkgid_idx ON packages(id);