// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumdb

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

func (pkg RepositoryPackage) RPMName() string {
	arch := pkg.Architecture
	if pkg.SourceRPM == "" {
		arch = "src"
	}
	return fmt.Sprintf("%s-%s-%s.%s.rpm", pkg.Name, pkg.Version, pkg.Release, arch)
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

func (db *RepositoryDB) AddPackage(ctx context.Context, pkg *RepositoryPackage) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	//nolint:gosec
	pkg.Tag = fmt.Sprintf("%x", md5.Sum([]byte(pkg.RPMName())))

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		// BE CAREFUL and respect the table's columns order !!
		"INSERT INTO packages VALUES(:tag, :id, :name, :upload_time, :build_time, :size, :architecture, :source_rpm, "+
			":version, :release, :groups, :license, :vendor, :summary, :description, :verified, :gpg_signature) "+
			"ON CONFLICT (tag) DO UPDATE SET id = :id, name = :name, upload_time = :upload_time, build_time = :build_time, "+
			"size = :size, architecture = :architecture, verified = :verified, source_rpm = :source_rpm, version = :version, "+
			"release = :release, groups = :groups, license = :license, vendor = :vendor, summary = :summary, "+
			"description = :description, gpg_signature = :gpg_signature",
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
		return fmt.Errorf("inventory package not inserted into database")
	}

	return nil
}

func (db *RepositoryDB) RemovePackage(ctx context.Context, id string) (bool, error) {
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

func (db *RepositoryDB) GetPackage(ctx context.Context, id string) (*RepositoryPackage, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM packages WHERE id = ? LIMIT 1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pkg := new(RepositoryPackage)

	if !rows.Next() {
		return nil, fmt.Errorf("failed to retrieve package with id %s", id)
	}
	if err := rows.StructScan(pkg); err != nil {
		return nil, err
	}

	return pkg, nil
}

func (db *RepositoryDB) GetPackageByTag(ctx context.Context, tag string) (*RepositoryPackage, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT * FROM packages WHERE tag = ? LIMIT 1", tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pkg := new(RepositoryPackage)

	if !rows.Next() {
		return nil, fmt.Errorf("failed to retrieve package with tag %s", tag)
	}
	if err := rows.StructScan(pkg); err != nil {
		return nil, err
	}

	return pkg, nil
}

type WalkPackageFunc func(*RepositoryPackage) error

func (db *RepositoryDB) WalkPackages(ctx context.Context, walkFn WalkPackageFunc) error {
	if walkFn == nil {
		return fmt.Errorf("no package walk function provided")
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
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return false, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT COUNT(id) FROM packages WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	count := 0

	if !rows.Next() {
		return false, fmt.Errorf("no rows found in packages table to count")
	}
	if err := rows.Scan(&count); err != nil {
		return false, err
	}

	return count > 0, nil
}

func (db *RepositoryDB) CountPackages(ctx context.Context) (int, error) {
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
