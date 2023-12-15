// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumdb

import (
	"context"
	"embed"
	"fmt"

	"go.ciq.dev/beskar/internal/pkg/sqlite"
	"gocloud.dev/blob"
)

//go:embed schema/metadata/*.sql
var metadataSchemas embed.FS

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
	*sqlite.DB
}

func OpenMetadataDB(ctx context.Context, bucket *blob.Bucket, dataDir string, repository string) (*MetadataDB, error) {
	db, err := sqlite.New(ctx, "metadata", sqlite.Storage{
		Bucket:             bucket,
		DataDir:            dataDir,
		Repository:         repository,
		SchemaFS:           metadataSchemas,
		SchemaGlob:         "schema/metadata/*.sql",
		Filename:           "metadata.db",
		CompressedFilename: "metadata.db.lz4",
	})
	if err != nil {
		return nil, err
	}

	return &MetadataDB{db}, nil
}

func (db *MetadataDB) AddPackage(ctx context.Context, pkg *PackageMetadata) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		// BE CAREFUL and respect the table's columns order !!
		"INSERT INTO packages VALUES(:name, :id, :meta_primary, :meta_filelists, :meta_other) "+
			"ON CONFLICT (name) DO UPDATE SET id = :id, meta_primary = :meta_primary, meta_filelists = :meta_filelists, meta_other = :meta_other",
		pkg,
	)
	db.Unlock()

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

func (db *MetadataDB) RemovePackage(ctx context.Context, id string) (bool, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return false, err
	}

	db.Lock()
	result, err := db.ExecContext(ctx, "DELETE FROM packages WHERE id = ?", id)
	db.Unlock()

	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected == 1, nil
}

func (db *MetadataDB) CountPackages(ctx context.Context) (int, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return 0, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT COUNT(name) FROM packages")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0

	if !rows.Next() {
		return 0, fmt.Errorf("no rows found in packages table to count")
	}
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

type WalkPackageMetadataFunc func(*PackageMetadata) error

func (db *MetadataDB) WalkPackageMetadata(ctx context.Context, walkFn WalkPackageMetadataFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no walk package function provided")
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
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
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"INSERT INTO extra_metadata VALUES(:type, :filename, :checksum, :open_checksum, :size, :open_size, :timestamp, :data) "+
			"ON CONFLICT (type) DO UPDATE SET filename = :filename, checksum = :checksum, open_checksum = :open_checksum, size = :size, "+
			"open_size = :open_size, timestamp = :timestamp, data = :data",
		extraRepodata,
	)
	db.Unlock()

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

func (db *MetadataDB) RemoveExtraMetadata(ctx context.Context, dataType string) (bool, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return false, err
	}

	db.Lock()
	result, err := db.ExecContext(ctx, "DELETE FROM extra_metadata WHERE type = ?", dataType)
	db.Unlock()

	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected == 1, nil
}

type WalkExtraMetadataFunc func(*ExtraMetadata) error

func (db *MetadataDB) WalkExtraMetadata(ctx context.Context, walkFn WalkExtraMetadataFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no walk extra repodata function provided")
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
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
