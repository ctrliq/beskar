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
	"gocloud.dev/blob"
)

const (
	metadataDBFile           = "metadata.db"
	metadataDBCompressedFile = "metadata.db.lz4"
)

//go:embed schema/metadata/*.sql
var MetadataSchemas embed.FS

type PackageMetadata struct {
	Name      string `db:"name"`
	ID        string `db:"id"`
	Primary   []byte `db:"meta_primary"`
	Filelists []byte `db:"meta_filelists"`
	Other     []byte `db:"meta_other"`
}

type ExtraMetadata struct {
	Type         string `db:"type"`
	Filename     string `db:"filename"`
	Checksum     string `db:"checksum"`
	OpenChecksum string `db:"open_checksum"`
	Size         uint64 `db:"size"`
	OpenSize     uint64 `db:"open_size"`
	Timestamp    int64  `db:"timestamp"`
	Data         []byte `db:"data"`
}

type MetadataDB struct {
	*sqlx.DB
	reference  atomic.Int32
	mutex      sync.RWMutex
	bucket     *blob.Bucket
	dataDir    string
	repository string
}

func OpenMetadataDB(ctx context.Context, bucket *blob.Bucket, dataDir string, repository string) (*MetadataDB, error) {
	metadataDB := &MetadataDB{
		bucket:     bucket,
		dataDir:    dataDir,
		repository: repository,
	}

	if err := metadataDB.open(ctx); err != nil {
		return nil, err
	}

	migrations, err := schema.FSMigrations(MetadataSchemas, "schema/metadata/*.sql")
	if err != nil {
		return nil, fmt.Errorf("while setting metadata DB migration: %w", err)
	}

	migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
	if err := migrator.Apply(metadataDB.DB, migrations); err != nil {
		return nil, fmt.Errorf("while applying metadata DB migration: %w", err)
	}

	return metadataDB, nil
}

func (db *MetadataDB) open(ctx context.Context) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.DB != nil {
		return nil
	}

	key := filepath.Join(db.repository, metadataDBCompressedFile)

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

func (db *MetadataDB) Path() string {
	return filepath.Join(db.dataDir, db.repository, metadataDBFile)
}

func (db *MetadataDB) Sync(ctx context.Context) error {
	key := filepath.Join(db.repository, metadataDBCompressedFile)

	remoteWriter, err := db.bucket.NewWriter(ctx, key, &blob.WriterOptions{})
	if err != nil {
		return fmt.Errorf("while initializing s3 object writer: %w", err)
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := push(db.Path(), remoteWriter); err != nil {
		_ = remoteWriter.Close()
		return fmt.Errorf("while pushing metadata database to s3 bucket: %w", err)
	}

	return remoteWriter.Close()
}

func (db *MetadataDB) Close(removeDB bool) error {
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

func (db *MetadataDB) AddPackage(ctx context.Context, pkg *PackageMetadata) error {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	db.mutex.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"INSERT INTO packages VALUES(:id, :name, :meta_primary, :meta_filelists, :meta_other) "+
			"ON CONFLICT (name) DO UPDATE SET id = :id, meta_primary = :meta_primary, meta_filelists = :meta_filelists, meta_other = :meta_other",
		pkg,
	)
	db.mutex.Unlock()

	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("package not inserted into database")
	}

	return nil
}

func (db *MetadataDB) CountPackages(ctx context.Context) (int, error) {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return 0, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT COUNT(name) AS name FROM packages")
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

type WalkPackageMetadataFunc func(*PackageMetadata) error

func (db *MetadataDB) WalkPackageMetadata(ctx context.Context, walkFn WalkPackageMetadataFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no walk package function provided")
	}

	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM packages")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		pkg := new(PackageMetadata)
		err := rows.StructScan(pkg)
		if err != nil {
			return err
		} else if err := walkFn(pkg); err != nil {
			return err
		}
	}

	return nil
}

func (db *MetadataDB) AddExtraMetadata(ctx context.Context, extraRepodata *ExtraMetadata) error {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	db.mutex.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"INSERT INTO extra_metadata VALUES(:type, :filename, :checksum, :open_checksum, :size, :open_size, :timestamp, :data) "+
			"ON CONFLICT (type) DO UPDATE SET filename = :filename, checksum = :checksum, open_checksum = :open_checksum, size = :size, "+
			"open_size = :open_size, timestamp = :timestamp, data = :data",
		extraRepodata,
	)
	db.mutex.Unlock()

	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("extra repodata not inserted into database")
	}

	return nil
}

type WalkExtraMetadataFunc func(*ExtraMetadata) error

func (db *MetadataDB) WalkExtraMetadata(ctx context.Context, walkFn WalkExtraMetadataFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no walk extra repodata function provided")
	}

	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM extra_metadata")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		extraMetadata := new(ExtraMetadata)
		err := rows.StructScan(extraMetadata)
		if err != nil {
			return err
		} else if err := walkFn(extraMetadata); err != nil {
			return err
		}
	}

	return nil
}
