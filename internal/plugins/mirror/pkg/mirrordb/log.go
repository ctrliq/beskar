// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrordb

import (
	"context"
	"embed"
	"fmt"
	"time"

	"go.ciq.dev/beskar/internal/pkg/sqlite"
	"gocloud.dev/blob"
)

const (
	LogError string = "ERROR"
	LogInfo  string = "INFO"
)

//go:embed schema/log/*.sql
var logSchemas embed.FS

type Log struct {
	ID      uint64 `db:"id"`
	Level   string `db:"level"`
	Date    int64  `db:"date"`
	Message string `db:"message"`
}

type LogDB struct {
	*sqlite.DB
}

func OpenLogDB(ctx context.Context, bucket *blob.Bucket, dataDir string, repository string) (*LogDB, error) {
	db, err := sqlite.New(ctx, "log", sqlite.Storage{
		Bucket:             bucket,
		DataDir:            dataDir,
		Repository:         repository,
		SchemaFS:           logSchemas,
		SchemaGlob:         "schema/log/*.sql",
		Filename:           "log.db",
		CompressedFilename: "log.db.lz4",
	})
	if err != nil {
		return nil, err
	}

	return &LogDB{db}, nil
}

func (db *LogDB) AddLog(ctx context.Context, level string, message string) error {
	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"INSERT INTO logs(date, level, message) VALUES(:date, :level, :message)",
		&Log{
			Date:    time.Now().UTC().Unix(),
			Level:   level,
			Message: message,
		},
	)
	db.Unlock()

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

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
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
