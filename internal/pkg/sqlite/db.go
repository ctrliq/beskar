// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/adlio/schema"
	"github.com/jmoiron/sqlx"
	"gocloud.dev/blob"

	// load sqlite driver
	_ "modernc.org/sqlite"
)

type DB struct {
	*sqlx.DB
	sync.RWMutex

	Reference atomic.Int32

	name    string
	path    string
	storage Storage
}

func New(ctx context.Context, name string, storage Storage) (*DB, error) {
	db := &DB{
		name:    name,
		storage: storage,
		path:    filepath.Join(storage.DataDir, storage.Repository, storage.Filename),
	}

	if err := db.Open(ctx); err != nil {
		return nil, err
	}

	if err := db.migrate(); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *DB) migrate() error {
	migrations, err := schema.FSMigrations(db.storage.SchemaFS, db.storage.SchemaGlob)
	if err != nil {
		return fmt.Errorf("while setting %s DB migration: %w", db.name, err)
	}

	migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
	if err := migrator.Apply(db.DB, migrations); err != nil {
		return fmt.Errorf("while applying %s DB migration: %w", db.name, err)
	}

	return nil
}

func (db *DB) Open(ctx context.Context) error {
	db.Lock()
	defer db.Unlock()

	if db.DB != nil {
		return nil
	}

	key := filepath.Join(db.storage.Repository, db.storage.CompressedFilename)

	dbDir := filepath.Dir(db.path)
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		return fmt.Errorf("while creating database directory %s: %w", dbDir, err)
	}

	_, err := os.Stat(db.path)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		remoteReader, err := db.storage.Bucket.NewReader(ctx, key, &blob.ReaderOptions{})
		if err == nil {
			defer remoteReader.Close()

			if err := pull(db.path, remoteReader); err != nil {
				return fmt.Errorf("while pulling log DB: %w", err)
			}
		}
	} else if err != nil {
		return err
	}

	db.DB, err = sqlx.Open("sqlite", db.path)
	if err != nil {
		return fmt.Errorf("while opening log DB %s: %w", db.path, err)
	}
	db.SetMaxOpenConns(1)

	return nil
}

func (db *DB) Path() string {
	return db.path
}

func (db *DB) Sync(ctx context.Context) error {
	key := filepath.Join(db.storage.Repository, db.storage.CompressedFilename)

	remoteWriter, err := db.storage.Bucket.NewWriter(ctx, key, &blob.WriterOptions{})
	if err != nil {
		return fmt.Errorf("while initializing s3 object writer: %w", err)
	}

	db.Lock()
	defer db.Unlock()

	if err := push(db.path, remoteWriter); err != nil {
		_ = remoteWriter.Close()
		return fmt.Errorf("while pushing log database to s3 bucket: %w", err)
	}

	return remoteWriter.Close()
}

func (db *DB) Close(removeDB bool) error {
	db.Lock()
	defer db.Unlock()

	if db.DB != nil && db.Reference.Load() == 0 {
		err := db.DB.Close()
		db.DB = nil
		if removeDB {
			if removeErr := os.Remove(db.path); removeErr != nil && err == nil {
				err = removeErr
			}
		}
		return err
	}

	return nil
}
