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

	"github.com/adlio/schema"
	"github.com/jmoiron/sqlx"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"gocloud.dev/blob"
	"google.golang.org/protobuf/proto"
)

const (
	statusDBFile           = "status.db"
	statusDBCompressedFile = "status.db.lz4"
)

//go:embed schema/status/*.sql
var StatusSchemas embed.FS

// events table
type Event struct {
	ID      string `db:"id"`
	Payload []byte `db:"payload"`
}

type Properties struct {
	Created    bool   `db:"created"`
	Mirror     bool   `db:"mirror"`
	MirrorURLs []byte `db:"mirror_urls"`
	GPGKey     []byte `db:"gpg_key"`
}

type Reposync struct {
	Syncing        bool  `db:"syncing"`
	LastSyncTime   int64 `db:"last_sync_time"`
	TotalPackages  int   `db:"total_packages"`
	SyncedPackages int   `db:"synced_packages"`
}

type StatusDB struct {
	*sqlx.DB
	reference  atomic.Int32
	mutex      sync.RWMutex
	bucket     *blob.Bucket
	dataDir    string
	repository string
}

func OpenStatusDB(ctx context.Context, bucket *blob.Bucket, dataDir string, repository string) (*StatusDB, error) {
	statusDB := &StatusDB{
		bucket:     bucket,
		dataDir:    dataDir,
		repository: repository,
	}

	if err := statusDB.open(ctx); err != nil {
		return nil, err
	}

	migrations, err := schema.FSMigrations(StatusSchemas, "schema/status/*.sql")
	if err != nil {
		return nil, fmt.Errorf("while setting status DB migration: %w", err)
	}

	migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
	if err := migrator.Apply(statusDB.DB, migrations); err != nil {
		return nil, fmt.Errorf("while applying status DB migration: %w", err)
	}

	return statusDB, nil
}

func (db *StatusDB) open(ctx context.Context) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.DB != nil {
		return nil
	}

	key := filepath.Join(db.repository, statusDBCompressedFile)

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
				return fmt.Errorf("while pulling repository DB: %w", err)
			}
		}
	} else if err != nil {
		return err
	}

	db.DB, err = sqlx.Open(driverName, dbPath)
	if err != nil {
		return fmt.Errorf("while opening repository DB %s: %w", dbPath, err)
	}
	db.SetMaxOpenConns(1)

	return nil
}

func (db *StatusDB) Path() string {
	return filepath.Join(db.dataDir, db.repository, statusDBFile)
}

func (db *StatusDB) Sync(ctx context.Context) error {
	key := filepath.Join(db.repository, statusDBCompressedFile)

	remoteWriter, err := db.bucket.NewWriter(ctx, key, &blob.WriterOptions{})
	if err != nil {
		return fmt.Errorf("while initializing s3 object writer: %w", err)
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := push(db.Path(), remoteWriter); err != nil {
		_ = remoteWriter.Close()
		return fmt.Errorf("while pushing status database to s3 bucket: %w", err)
	}

	return remoteWriter.Close()
}

func (db *StatusDB) Close(removeDB bool) error {
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

func (db *StatusDB) AddEvent(ctx context.Context, event *eventv1.EventPayload) error {
	payload, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("while marshalling event: %w", err)
	}

	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	db.mutex.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"INSERT INTO events VALUES(:id, :payload) ON CONFLICT (id) DO UPDATE SET payload = :payload",
		&Event{
			ID:      event.Digest,
			Payload: payload,
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
		return fmt.Errorf("event not inserted into status database")
	}

	return nil
}

func (db *StatusDB) RemoveEvent(ctx context.Context, id string) error {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	db.mutex.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"DELETE FROM events WHERE id = :id",
		&Event{ID: id},
	)
	db.mutex.Unlock()

	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("event id %s not present in status database", id)
	}

	return nil
}

type WalkEventsFunc func(*eventv1.EventPayload) error

func (db *StatusDB) WalkEvents(ctx context.Context, walkFn WalkEventsFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no walk events function provided")
	}

	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM events")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		eventRecord := new(Event)
		err := rows.StructScan(eventRecord)
		if err != nil {
			return err
		}
		event := new(eventv1.EventPayload)
		if err := proto.Unmarshal(eventRecord.Payload, event); err != nil {
			return fmt.Errorf("while unmarshalling event: %w", err)
		} else if err := walkFn(event); err != nil {
			return err
		}
	}

	return nil
}

func (db *StatusDB) CountEnvents(ctx context.Context) (int, error) {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return 0, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT COUNT(id) as id FROM events")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0

	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return 0, err
		}
	}

	return count, nil
}

func (db *StatusDB) GetProperties(ctx context.Context) (*Properties, error) {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT created, mirror, mirror_urls, gpg_key FROM properties WHERE id = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	properties := new(Properties)

	for rows.Next() {
		err := rows.StructScan(properties)
		if err != nil {
			return nil, err
		}
	}

	return properties, nil
}

func (db *StatusDB) UpdateProperties(ctx context.Context, properties *Properties) error {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	db.mutex.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"UPDATE properties SET created = :created, mirror = :mirror, mirror_urls = :mirror_urls, gpg_key = :gpg_key WHERE id = 1",
		properties,
	)
	db.mutex.Unlock()

	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("properties not updated in status database")
	}

	return nil
}

func (db *StatusDB) GetReposync(ctx context.Context) (*Reposync, error) {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(
		ctx,
		"SELECT syncing, last_sync_time, total_packages, synced_packages FROM reposync WHERE id = 1",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reposync := new(Reposync)

	for rows.Next() {
		err := rows.StructScan(reposync)
		if err != nil {
			return nil, err
		}
	}

	return reposync, nil
}

func (db *StatusDB) UpdateReposync(ctx context.Context, reposync *Reposync) error {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	db.mutex.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"UPDATE reposync SET syncing = :syncing, last_sync_time = :last_sync_time, "+
			"total_packages = :total_packages, synced_packages = :synced_packages "+
			"WHERE id = 1",
		reposync,
	)
	db.mutex.Unlock()

	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("reposync not updated in status database")
	}

	return nil
}
