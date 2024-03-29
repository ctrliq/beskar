// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrordb

import (
	"context"
	"crypto/md5" //nolint:gosec
	"embed"
	"encoding/hex"
	"fmt"

	"go.ciq.dev/beskar/internal/pkg/sqlite"
	"gocloud.dev/blob"
)

//go:embed schema/repository/*.sql
var repositorySchemas embed.FS

type RepositoryFile struct {
	Tag           string `db:"tag"`
	Name          string `db:"name"`
	Reference     string `db:"reference"`
	Parent        string `db:"parent"`
	Link          string `db:"link"`
	ModifiedTime  int64  `db:"modified_time"`
	Mode          uint32 `db:"mode"`
	Size          uint64 `db:"size"`
	ConfigID      uint64 `db:"config_id"`
	LinkReference string `db:"link_reference"`
}

type RepositoryDB struct {
	*sqlite.DB
}

func OpenRepositoryDB(ctx context.Context, bucket *blob.Bucket, dataDir string, repository string) (*RepositoryDB, error) {
	db, err := sqlite.New(ctx, "repository", sqlite.Storage{
		Bucket:             bucket,
		DataDir:            dataDir,
		Repository:         repository,
		SchemaFS:           repositorySchemas,
		SchemaGlob:         "schema/repository/*.sql",
		Filename:           "repository.db",
		CompressedFilename: "repository.db.lz4",
	})
	if err != nil {
		return nil, err
	}

	return &RepositoryDB{db}, nil
}

func (db *RepositoryDB) AddFile(ctx context.Context, file *RepositoryFile) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	//nolint:gosec
	s := md5.Sum([]byte(file.Reference))
	file.Tag = hex.EncodeToString(s[:])

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		// BE CAREFUL and respect the table's columns order !!
		"INSERT INTO files VALUES(:tag, :name, :reference, :parent, :link, :modified_time, :mode, :size, :config_id, :link_reference) "+
			"ON CONFLICT (tag) DO UPDATE SET name = :name, reference = :reference, parent = :parent, link = :link, modified_time = :modified_time, mode = :mode, size = :size, config_id = :config_id, link_reference = :link_reference",
		file,
	)
	db.Unlock()

	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("file information not inserted into database")
	}

	return nil
}

func (db *RepositoryDB) RemoveFile(ctx context.Context, tag string) (bool, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return false, err
	}

	db.Lock()
	result, err := db.ExecContext(ctx, "DELETE FROM files WHERE tag = ?", tag)
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

func (db *RepositoryDB) GetFileByTag(ctx context.Context, tag string) (*RepositoryFile, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM files WHERE tag = ? LIMIT 1", tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	file := new(RepositoryFile)

	if !rows.Next() {
		return nil, sqlite.ErrNoEntryFound
	}
	if err := rows.StructScan(file); err != nil {
		return nil, err
	}

	return file, nil
}

func (db *RepositoryDB) GetFileByName(ctx context.Context, name string) (*RepositoryFile, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM files WHERE name = ? LIMIT 1", name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	file := new(RepositoryFile)

	if !rows.Next() {
		return nil, sqlite.ErrNoEntryFound
	}
	if err := rows.StructScan(file); err != nil {
		return nil, err
	}

	return file, nil
}

func (db *RepositoryDB) GetFileByReference(ctx context.Context, reference string) (*RepositoryFile, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM files WHERE reference = ? LIMIT 1", reference)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	file := new(RepositoryFile)

	if !rows.Next() {
		return nil, sqlite.ErrNoEntryFound
	}
	if err := rows.StructScan(file); err != nil {
		return nil, err
	}

	return file, nil
}

func (db *RepositoryDB) DeleteFileByName(ctx context.Context, name string) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Lock()
	result, err := db.ExecContext(ctx, "DELETE FROM files WHERE name = ?", name)
	db.Unlock()

	if err != nil {
		return err
	}

	_, err = result.RowsAffected()
	if err != nil {
		return err
	}

	return nil
}

func (db *RepositoryDB) DeleteFilesByMode(ctx context.Context, mode uint32) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Lock()
	result, err := db.ExecContext(ctx, "DELETE FROM files WHERE mode = ?", mode)
	db.Unlock()

	if err != nil {
		return err
	}

	_, err = result.RowsAffected()
	if err != nil {
		return err
	}

	return nil
}

type WalkFileFunc func(*RepositoryFile) error

func (db *RepositoryDB) WalkFiles(ctx context.Context, walkFn WalkFileFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no file walk function provided")
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM files")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		file := new(RepositoryFile)
		err := rows.StructScan(file)
		if err != nil {
			return err
		} else if err := walkFn(file); err != nil {
			return err
		}
	}

	return nil
}

func (db *RepositoryDB) WalkSymlinks(ctx context.Context, walkFn WalkFileFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no file walk function provided")
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM files WHERE link <> '' ORDER BY LENGTH(name) ASC")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		file := new(RepositoryFile)
		err := rows.StructScan(file)
		if err != nil {
			return err
		} else if err := walkFn(file); err != nil {
			return err
		}
	}

	return nil
}

func (db *RepositoryDB) WalkFilesByParent(ctx context.Context, parent string, walkFn WalkFileFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no file walk function provided")
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM files WHERE parent = ?", parent)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		file := new(RepositoryFile)
		err := rows.StructScan(file)
		if err != nil {
			return err
		} else if err := walkFn(file); err != nil {
			return err
		}
	}

	return nil
}

func (db *RepositoryDB) WalkFilesByConfigID(ctx context.Context, configID uint64, walkFn WalkFileFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no file walk function provided")
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM files WHERE config_id = ?", configID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		file := new(RepositoryFile)
		err := rows.StructScan(file)
		if err != nil {
			return err
		} else if err := walkFn(file); err != nil {
			return err
		}
	}

	return nil
}

type WalkStringFunc func(*string) error

func (db *RepositoryDB) WalkFilesByDistinctParent(ctx context.Context, walkFn WalkStringFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no file walk function provided")
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	rows, err := db.QueryxContext(ctx, "SELECT DISTINCT parent FROM files")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		parent := new(string)
		err := rows.Scan(parent)
		if err != nil {
			return err
		} else if err := walkFn(parent); err != nil {
			return err
		}
	}

	return nil
}

func (db *RepositoryDB) CountFiles(ctx context.Context) (int, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return 0, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT COUNT(tag) FROM files")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0

	if !rows.Next() {
		return 0, fmt.Errorf("no rows found in files table to count")
	}
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}
