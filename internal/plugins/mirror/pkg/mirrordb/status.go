// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrordb

import (
	"context"
	"embed"
	"fmt"

	"go.ciq.dev/beskar/internal/pkg/sqlite"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"gocloud.dev/blob"
	"google.golang.org/protobuf/proto"
)

//go:embed schema/status/*.sql
var statusSchemas embed.FS

// events table
type Event struct {
	ID      string `db:"id"`
	Payload []byte `db:"payload"`
}

type Properties struct {
	Created    bool   `db:"created"`
	Mirror     bool   `db:"mirror"`
	MirrorURLs []byte `db:"mirror_urls"`
}

type Sync struct {
	Syncing     bool   `db:"syncing"`
	StartTime   int64  `db:"start_time"`
	EndTime     int64  `db:"end_time"`
	TotalFiles  int    `db:"total_files"`
	SyncedFiles int    `db:"synced_files"`
	SyncError   string `db:"sync_error"`
}

type StatusDB struct {
	*sqlite.DB
}

func OpenStatusDB(ctx context.Context, bucket *blob.Bucket, dataDir string, repository string) (*StatusDB, error) {
	db, err := sqlite.New(ctx, "status", sqlite.Storage{
		Bucket:             bucket,
		DataDir:            dataDir,
		Repository:         repository,
		SchemaFS:           statusSchemas,
		SchemaGlob:         "schema/status/*.sql",
		Filename:           "status.db",
		CompressedFilename: "status.db.lz4",
	})
	if err != nil {
		return nil, err
	}

	return &StatusDB{db}, nil
}

func (db *StatusDB) AddEvent(ctx context.Context, event *eventv1.EventPayload) error {
	payload, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("while marshalling event: %w", err)
	}

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"INSERT INTO events VALUES(:id, :payload) ON CONFLICT (id) DO UPDATE SET payload = :payload",
		&Event{
			ID:      fmt.Sprintf("%s:%s", event.Digest, event.Action),
			Payload: payload,
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
		return fmt.Errorf("event not inserted into status database")
	}

	return nil
}

func (db *StatusDB) RemoveEvent(ctx context.Context, event *eventv1.EventPayload) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	id := fmt.Sprintf("%s:%s", event.Digest, event.Action)

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"DELETE FROM events WHERE id = :id",
		&Event{ID: id},
	)
	db.Unlock()

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

	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
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

func (db *StatusDB) CountEvents(ctx context.Context) (int, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return 0, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT COUNT(id) FROM events")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0

	if !rows.Next() {
		return 0, fmt.Errorf("no rows found in events table to count")
	}
	if err := rows.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (db *StatusDB) GetProperties(ctx context.Context) (*Properties, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(ctx, "SELECT created, mirror, mirror_urls FROM properties WHERE id = 1")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	properties := new(Properties)

	if !rows.Next() {
		return nil, fmt.Errorf("failed to retrieve repository properties")
	}
	if err := rows.StructScan(properties); err != nil {
		return nil, err
	}

	return properties, nil
}

func (db *StatusDB) UpdateProperties(ctx context.Context, properties *Properties) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"UPDATE properties SET created = :created, mirror = :mirror, mirror_urls = :mirror_urls WHERE id = 1",
		properties,
	)
	db.Unlock()

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

func (db *StatusDB) SetCreatedProperty(ctx context.Context) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Lock()
	result, err := db.ExecContext(
		ctx,
		"UPDATE properties SET created = true WHERE id = 1",
	)
	db.Unlock()

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

func (db *StatusDB) GetSync(ctx context.Context) (*Sync, error) {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return nil, err
	}

	rows, err := db.QueryxContext(
		ctx,
		"SELECT syncing, start_time, end_time, total_files, synced_files, sync_error FROM sync WHERE id = 1",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sync := new(Sync)

	if !rows.Next() {
		return nil, fmt.Errorf("failed to retrieve sync data")
	}
	if err := rows.StructScan(sync); err != nil {
		return nil, err
	}

	return sync, nil
}

func (db *StatusDB) UpdateSync(ctx context.Context, sync *Sync) error {
	db.Reference.Add(1)
	defer db.Reference.Add(-1)

	if err := db.Open(ctx); err != nil {
		return err
	}

	db.Lock()
	result, err := db.NamedExecContext(
		ctx,
		"UPDATE sync SET syncing = :syncing, start_time = :start_time, end_time = :end_time, "+
			"total_files = :total_files, synced_files = :synced_files, sync_error = :sync_error "+
			"WHERE id = 1",
		sync,
	)
	db.Unlock()

	if err != nil {
		return err
	}

	inserted, err := result.RowsAffected()
	if err != nil {
		return err
	} else if inserted != 1 {
		return fmt.Errorf("sync not updated in status database")
	}

	return nil
}
