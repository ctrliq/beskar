CREATE TABLE IF NOT EXISTS extra_metadata (
	type TEXT PRIMARY KEY,
	filename TEXT,
	checksum TEXT,
	open_checksum TEXT,
	size INTEGER,
	open_size INTEGER,
	timestamp INTEGER,
	data BLOB
);