ALTER TABLE reposync RENAME COLUMN last_sync_time TO start_time;
ALTER TABLE reposync ADD end_time INTEGER DEFAULT 0 NOT NULL;
ALTER TABLE reposync ADD sync_error TEXT DEFAULT '' NOT NULL;