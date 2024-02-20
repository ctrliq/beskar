CREATE TABLE IF NOT EXISTS properties (
    id INTEGER PRIMARY KEY,
    created BOOLEAN,
    mirror BOOLEAN,
    mirror_configs BLOB,
    web_config BLOB
);

INSERT INTO properties VALUES(1, false, false, '', '');