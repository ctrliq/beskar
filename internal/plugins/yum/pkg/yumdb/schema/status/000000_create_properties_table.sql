CREATE TABLE IF NOT EXISTS properties (
    id INTEGER PRIMARY KEY,
    created BOOLEAN,
	mirror BOOLEAN,
    mirror_urls BLOB,
    gpg_key BLOB
);

INSERT INTO properties VALUES(1, false, false, '', '');