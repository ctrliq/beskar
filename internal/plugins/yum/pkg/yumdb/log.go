// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumdb

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/adlio/schema"
	"github.com/jmoiron/sqlx"
	"gocloud.dev/blob"
)

const (
	logDBFile           = "log.db"
	logDBCompressedFile = "log.db.lz4"
)

const (
	LogError string = "ERROR"
	LogInfo  string = "INFO"
)

//go:embed schema/log/*.sql
var LogSchemas embed.FS

type Log struct {
	ID      uint64 `db:"id"`
	Level   string `db:"level"`
	Date    int64  `db:"date"`
	Message string `db:"message"`
}

type LogDB struct {
	*sqlx.DB
	reference  atomic.Int32
	mutex      sync.RWMutex
	bucket     *blob.Bucket
	dataDir    string
	repository string
}

func OpenLogDB(ctx context.Context, bucket *blob.Bucket, dataDir string, repository string) (*LogDB, error) {
	logDB := &LogDB{
		bucket:     bucket,
		dataDir:    dataDir,
		repository: repository,
	}

	if err := logDB.open(ctx); err != nil {
		return nil, err
	}

	migrations, err := schema.FSMigrations(LogSchemas, "schema/log/*.sql")
	if err != nil {
		return nil, fmt.Errorf("while setting log DB migration: %w", err)
	}

	migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
	if err := migrator.Apply(logDB.DB, migrations); err != nil {
		return nil, fmt.Errorf("while applying log DB migration: %w", err)
	}

	return logDB, nil
}

func (db *LogDB) open(ctx context.Context) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.DB != nil {
		return nil
	}

	key := filepath.Join(db.repository, logDBCompressedFile)

	dbPath := db.Path()
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0o700); err != nil {
		return fmt.Errorf("while creating database directory %s: %w", dbDir, err)
	}

	_, err := os.Stat(dbPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		remoteReader, err := db.bucket.NewReader(ctx, key, &blob.ReaderOptions{})
		if err == nil {
			defer remoteReader.Close()

			if err := pull(dbPath, remoteReader); err != nil {
				return fmt.Errorf("while pulling log DB: %w", err)
			}
		}
	} else if err != nil {
		return err
	}

	db.DB, err = sqlx.Open(driverName, dbPath)
	if err != nil {
		return fmt.Errorf("while opening log DB %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)

	return nil
}

func (db *LogDB) Path() string {
	return filepath.Join(db.dataDir, db.repository, logDBFile)
}

func (db *LogDB) Sync(ctx context.Context) error {
	key := filepath.Join(db.repository, logDBCompressedFile)

	remoteWriter, err := db.bucket.NewWriter(ctx, key, &blob.WriterOptions{})
	if err != nil {
		return fmt.Errorf("while initializing s3 object writer: %w", err)
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := push(db.Path(), remoteWriter); err != nil {
		_ = remoteWriter.Close()
		return fmt.Errorf("while pushing log database to s3 bucket: %w", err)
	}

	return remoteWriter.Close()
}

func (db *LogDB) Close(removeDB bool) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.DB != nil && db.reference.Load() == 0 {
		err := db.DB.Close()
		db.DB = nil
		if removeDB {
			if removeErr := os.Remove(db.Path()); removeErr != nil && err == nil {
				err = removeErr
			}
		}
		return err
	}

	return nil
}

func (db *LogDB) AddLog(ctx context.Context, level string, message string) error {
	if err := db.open(ctx); err != nil {
		return err
	}

	db.reference.Add(1)
	defer db.reference.Add(-1)

	db.mutex.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"INSERT INTO logs(date, level, message) VALUES(:date, :level, :message)",
		&Log{
			Date:    time.Now().UTC().Unix(),
			Level:   level,
			Message: message,
		},
	)
	db.mutex.Unlock()

	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("log not inserted into log database")
	}

	return nil
}

type WalkLogFunc func(*Log) error

func (db *LogDB) WalkLogs(ctx context.Context, walkFn WalkLogFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no log walk function provided")
	}

	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM logs")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		log := new(Log)
		err := rows.StructScan(log)
		if err != nil {
			return err
		} else if err := walkFn(log); err != nil {
			return err
		}
	}

	return nil
}
