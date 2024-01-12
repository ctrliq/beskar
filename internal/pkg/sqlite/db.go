// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
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

var ErrNoEntryFound = errors.New("no entry found")

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
		return fmt.Errorf("while setting %s database migration: %w", db.name, err)
	}

	migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
	if err := migrator.Apply(db.DB, migrations); err != nil {
		return fmt.Errorf("while applying %s database migration: %w", db.name, err)
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
	migrate := false

	_, err := os.Stat(db.path)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		dbDir := filepath.Dir(db.path)
		if err := os.MkdirAll(dbDir, 0o700); err != nil {
			return fmt.Errorf("while creating database directory %s: %w", dbDir, err)
		}

		remoteReader, err := db.storage.Bucket.NewReader(ctx, key, &blob.ReaderOptions{})
		if err == nil {
			defer remoteReader.Close()

			if err := pull(db.path, remoteReader); err != nil {
				return fmt.Errorf("while pulling %s database: %w", db.name, err)
			}
		}
		migrate = err != nil
	} else if err != nil {
		return err
	}

	db.DB, err = sqlx.Open("sqlite", db.path)
	if err != nil {
		return fmt.Errorf("while opening %s database %s: %w", db.name, db.path, err)
	}
	db.SetMaxOpenConns(1)

	if migrate {
		if err := db.migrate(); err != nil {
			return err
		}
	}

	return nil
}

func (db *DB) Path() string {
	return db.path
}

func (db *DB) Sync(ctx context.Context) error {
	key := filepath.Join(db.storage.Repository, db.storage.CompressedFilename)

	remoteWriter, err := db.storage.Bucket.NewWriter(ctx, key, &blob.WriterOptions{})
	if err != nil {
		return fmt.Errorf("while initializing object writer: %w", err)
	}

	db.Lock()
	defer db.Unlock()

	if err := push(db.path, remoteWriter); err != nil {
		_ = remoteWriter.Close()
		return fmt.Errorf("while pushing %s database to storage bucket: %w", db.name, err)
	}

	return remoteWriter.Close()
}

func (db *DB) Close(removeLocalDB bool) error {
	db.Lock()
	defer db.Unlock()

	if db.DB != nil && db.Reference.Load() == 0 {
		err := db.DB.Close()
		db.DB = nil
		if removeLocalDB {
			if removeErr := os.Remove(db.path); removeErr != nil && err == nil {
				err = removeErr
			}
		}
		return err
	}

	return nil
}

func (db *DB) Delete(ctx context.Context) error {
	db.Lock()
	defer db.Unlock()

	key := filepath.Join(db.storage.Repository, db.storage.CompressedFilename)

	if err := db.storage.Bucket.Delete(ctx, key); err != nil {
		return fmt.Errorf("while deleting %s database on object storage: %w", db.name, err)
	}

	return nil
}
