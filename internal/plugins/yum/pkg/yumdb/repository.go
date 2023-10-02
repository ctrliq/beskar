// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumdb

import (
	"context"
	"crypto/md5" //nolint:gosec
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
	repositoryDBFile           = "repository.db"
	repositoryDBCompressedFile = "repository.db.lz4"
)

//go:embed schema/repository/*.sql
var RepositorySchemas embed.FS

type RepositoryPackage struct {
	Tag          string `db:"tag"`
	ID           string `db:"id"`
	Name         string `db:"name"`
	UploadTime   int64  `db:"upload_time"`
	BuildTime    int64  `db:"build_time"`
	Size         uint64 `db:"size"`
	Architecture string `db:"architecture"`
	SourceRPM    string `db:"source_rpm"`
	Version      string `db:"version"`
	Release      string `db:"release"`
	Groups       string `db:"groups"`
	License      string `db:"license"`
	Vendor       string `db:"vendor"`
	Summary      string `db:"summary"`
	Description  string `db:"description"`
	Verified     bool   `db:"verified"`
	GPGSignature string `db:"gpg_signature"`
}

type RepositoryDB struct {
	*sqlx.DB
	reference  atomic.Int32
	mutex      sync.RWMutex
	bucket     *blob.Bucket
	dataDir    string
	repository string
}

func OpenRepositoryDB(ctx context.Context, bucket *blob.Bucket, dataDir string, repository string) (*RepositoryDB, error) {
	repoDB := &RepositoryDB{
		bucket:     bucket,
		dataDir:    dataDir,
		repository: repository,
	}

	if err := repoDB.open(ctx); err != nil {
		return nil, err
	}

	migrations, err := schema.FSMigrations(RepositorySchemas, "schema/repository/*.sql")
	if err != nil {
		return nil, fmt.Errorf("while setting repository DB migration: %w", err)
	}

	migrator := schema.NewMigrator(schema.WithDialect(schema.SQLite))
	if err := migrator.Apply(repoDB.DB, migrations); err != nil {
		return nil, fmt.Errorf("while applying repository DB migration: %w", err)
	}

	return repoDB, nil
}

func (db *RepositoryDB) open(ctx context.Context) error {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	if db.DB != nil {
		return nil
	}

	key := filepath.Join(db.repository, repositoryDBCompressedFile)

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

func (db *RepositoryDB) Path() string {
	return filepath.Join(db.dataDir, db.repository, repositoryDBFile)
}

func (db *RepositoryDB) Sync(ctx context.Context) error {
	key := filepath.Join(db.repository, repositoryDBCompressedFile)

	remoteWriter, err := db.bucket.NewWriter(ctx, key, &blob.WriterOptions{})
	if err != nil {
		return fmt.Errorf("while initializing object store writer: %w", err)
	}

	db.mutex.Lock()
	defer db.mutex.Unlock()

	if err := push(db.Path(), remoteWriter); err != nil {
		_ = remoteWriter.Close()
		return fmt.Errorf("while pushing repository database to object store: %w", err)
	}

	return remoteWriter.Close()
}

func (db *RepositoryDB) Close(removeDB bool) error {
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

func (db *RepositoryDB) AddPackage(ctx context.Context, pkg *RepositoryPackage) error {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return err
	}

	arch := pkg.Architecture
	if pkg.SourceRPM == "" {
		arch = "src"
	}

	rpmName := fmt.Sprintf("%s-%s-%s.%s.rpm", pkg.Name, pkg.Version, pkg.Release, arch)
	//nolint:gosec
	pkg.Tag = fmt.Sprintf("%x", md5.Sum([]byte(rpmName)))

	db.mutex.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"INSERT INTO packages VALUES(:tag, :id, :name, :upload_time, :build_time, :size, :architecture, :source_rpm, "+
			":version, :release, :groups, :license, :vendor, :summary, :description, :verified, :gpg_signature) "+
			"ON CONFLICT (tag) DO UPDATE SET id = :id, name = :name, upload_time = :upload_time, build_time = :build_time, "+
			"size = :size, architecture = :architecture, verified = :verified, source_rpm = :source_rpm, version = :version, "+
			"release = :release, groups = :groups, license = :license, vendor = :vendor, summary = :summary, "+
			"description = :description, gpg_signature = :gpg_signature",
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
		return fmt.Errorf("inventory package not inserted into database")
	}

	return nil
}

func (db *RepositoryDB) GetPackage(ctx context.Context, id string) (*RepositoryPackage, error) {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM packages WHERE id = ? LIMIT 1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pkg := new(RepositoryPackage)

	for rows.Next() {
		if err := rows.Scan(pkg); err != nil {
			return nil, err
		}
	}

	return pkg, nil
}

type WalkPackageFunc func(*RepositoryPackage) error

func (db *RepositoryDB) WalkPackages(ctx context.Context, walkFn WalkPackageFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no package walk function provided")
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
		pkg := new(RepositoryPackage)
		err := rows.StructScan(pkg)
		if err != nil {
			return err
		} else if err := walkFn(pkg); err != nil {
			return err
		}
	}

	return nil
}

func (db *RepositoryDB) HasPackageID(ctx context.Context, id string) (bool, error) {
	db.reference.Add(1)
	defer db.reference.Add(-1)

	if err := db.open(ctx); err != nil {
		return false, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT COUNT(id) FROM packages WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	count := 0

	for rows.Next() {
		if err := rows.Scan(&count); err != nil {
			return false, err
		}
	}

	return count > 0, nil
}
