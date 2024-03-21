CREATE TABLE IF NOT EXISTS files (
    tag TEXT PRIMARY KEY,
    name TEXT,
    reference TEXT,
    parent TEXT,
    link TEXT,
    modified_time INTEGER,
    mode INTEGER,
    size INTEGER,
    config_id INTEGER
);

CREATE INDEX files_name_idx ON files(name);
CREATE INDEX files_reference_idx ON files(reference);
CREATE INDEX files_parent_idx ON files(parent);
CREATE INDEX files_config_id_idx ON files(config_id);