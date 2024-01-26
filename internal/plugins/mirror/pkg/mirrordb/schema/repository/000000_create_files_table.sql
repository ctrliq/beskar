CREATE TABLE IF NOT EXISTS files (
    tag TEXT PRIMARY KEY,
    name TEXT,
    link TEXT,
    modified_time INTEGER,
    mode INTEGER,
    size INTEGER
);

CREATE INDEX filename_idx ON files(name);