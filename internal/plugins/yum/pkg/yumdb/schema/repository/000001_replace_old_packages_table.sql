DROP TABLE IF EXISTS old_packages;
DROP TRIGGER IF EXISTS package_changes_trigger;

CREATE TABLE IF NOT EXISTS package_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tag TEXT,
    name TEXT,
    package_id TEXT,
    new_package_id TEXT,
    created_at INTEGER,
    updated BOOLEAN,
    deleted BOOLEAN
);

CREATE TRIGGER IF NOT EXISTS package_update_trigger
    AFTER UPDATE ON packages
    WHEN old.id <> new.id
BEGIN
    INSERT INTO package_history (
        tag,
        name,
        package_id,
        new_package_id,
        created_at,
        updated
)
VALUES
    (
        old.tag,
        old.name || '-' || old.version || '-' || old.release || '.' || old.architecture || '.rpm',
        old.id,
        new.id,
        UNIXEPOCH(),
        true
    );
END;

CREATE TRIGGER IF NOT EXISTS package_delete_trigger
    AFTER DELETE ON packages
BEGIN
    INSERT INTO package_history (
        tag,
        name,
        package_id,
        created_at,
        deleted
)
VALUES
    (
        old.tag,
        old.name || '-' || old.version || '-' || old.release || '.' || old.architecture || '.rpm',
        old.id,
        UNIXEPOCH(),
        true
    );
END;