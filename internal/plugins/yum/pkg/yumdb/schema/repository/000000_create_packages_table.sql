CREATE TABLE IF NOT EXISTS packages (
    tag TEXT PRIMARY KEY,
	id TEXT,
	name TEXT,
	upload_time INTEGER,
    build_time INTEGER,
    size INTEGER,
	architecture TEXT,
    source_rpm TEXT,
    version TEXT,
    release TEXT,
    groups TEXT,
    license TEXT,
    vendor TEXT,
    summary TEXT,
    description TEXT,
    verified BOOLEAN,
    gpg_signature TEXT
);

CREATE INDEX pkgid_idx ON packages(id);

CREATE TABLE IF NOT EXISTS old_packages (
    tag TEXT PRIMARY KEY,
	id TEXT,
	name TEXT,
	upload_time INTEGER,
    build_time INTEGER,
    size INTEGER,
	architecture TEXT,
    source_rpm TEXT,
    version TEXT,
    release TEXT,
    groups TEXT,
    license TEXT,
    vendor TEXT,
    summary TEXT,
    description TEXT,
    verified BOOLEAN,
    gpg_signature TEXT
);

CREATE INDEX old_pkgid_idx ON old_packages(id);

CREATE TRIGGER IF NOT EXISTS package_changes_trigger
    AFTER UPDATE ON packages
    WHEN old.id <> new.id
BEGIN
    INSERT INTO old_packages (
        tag,
        id,
        name,
        upload_time,
        build_time,
        size,
        architecture,
        source_rpm,
        version,
        release,
        groups,
        license,
        vendor,
        summary,
        description,
        verified,
        gpg_signature
)
VALUES
    (
        old.tag,
        old.id,
        old.name,
        old.upload_time,
        old.build_time,
        old.size,
        old.architecture,
        old.source_rpm,
        old.version,
        old.release,
        old.groups,
        old.license,
        old.vendor,
        old.summary,
        old.description,
        old.verified,
        old.gpg_signature
    );
END;