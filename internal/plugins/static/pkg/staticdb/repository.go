// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package staticdb

import (
	"context"
	"crypto/md5" //nolint:gosec
	"embed"
	"fmt"

	"go.ciq.dev/beskar/internal/pkg/sqlite"
	"gocloud.dev/blob"
)

//go:embed schema/repository/*.sql
var repositorySchemas embed.FS

type RepositoryFile struct {
	Tag        string `db:"tag"`
	ID         string `db:"id"`
	Name       string `db:"name"`
	UploadTime int64  `db:"upload_time"`
	Size       uint64 `db:"size"`
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
	file.Tag = fmt.Sprintf("%x", md5.Sum([]byte(file.Name)))

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		// BE CAREFUL and respect the table's columns order !!
		"INSERT INTO files VALUES(:tag, :id, :name, :upload_time, :size) "+
			"ON CONFLICT (tag) DO UPDATE SET id = :id, name = :name, upload_time = :upload_time, size = :size",
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

	for rows.Next() {
		if err := rows.StructScan(file); err != nil {
			return nil, err
		}
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

	for rows.Next() {
		if err := rows.StructScan(file); err != nil {
			return nil, err
		}
	}

	return file, nil
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
