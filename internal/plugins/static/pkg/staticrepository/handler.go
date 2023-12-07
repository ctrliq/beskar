// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package staticrepository

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"go.ciq.dev/beskar/internal/pkg/repository"
	"go.ciq.dev/beskar/internal/plugins/static/pkg/staticdb"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"go.ciq.dev/beskar/pkg/orasfile"
)

type Handler struct {
	*repository.RepoHandler

	repoDir string

	logger *slog.Logger

	dbMutex      sync.Mutex
	repositoryDB *staticdb.RepositoryDB
	logDB        *staticdb.LogDB
	statusDB     *staticdb.StatusDB
}

func NewHandler(logger *slog.Logger, repoHandler *repository.RepoHandler) repository.Handler {
	return &Handler{
		RepoHandler: repoHandler,
		repoDir:     filepath.Join(repoHandler.Params.Dir, repoHandler.Repository),
		logger:      logger,
	}
}

func (h *Handler) QueueEvent(event *eventv1.EventPayload, store bool) error {
	ctx := context.Background()

	if store {
		db, err := h.getStatusDB(ctx)
		if err != nil {
			h.logger.Error("status database event", "digest", event.Digest, "mediatype", event.Mediatype, "error", err.Error())
			return err
		} else if err := db.AddEvent(ctx, event); err != nil {
			h.logger.Error("add event in status database", "digest", event.Digest, "mediatype", event.Mediatype, "error", err.Error())
			return err
		} else if err := db.Sync(ctx); err != nil {
			h.logger.Error("sync status database", "digest", event.Digest, "mediatype", event.Mediatype, "error", err.Error())
			return err
		}
	}

	h.logger.Info("queued event", "digest", event.Digest)

	h.EnqueueEvent(event)

	return nil
}

func (h *Handler) cleanup() {
	h.dbMutex.Lock()

	if h.logDB != nil {
		h.logDB.Close(true)
	}
	if h.statusDB != nil {
		h.statusDB.Close(true)
	}
	if h.repositoryDB != nil {
		h.repositoryDB.Close(true)
	}

	h.dbMutex.Unlock()

	close(h.Queued)
	h.Params.Remove(h.Repository)
	_ = os.RemoveAll(h.repoDir)
}

func (h *Handler) getStatusDB(ctx context.Context) (*staticdb.StatusDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.statusDB != nil {
		return h.statusDB, nil
	}

	db, err := staticdb.OpenStatusDB(ctx, h.Params.Bucket, h.repoDir, h.Repository)
	if err != nil {
		return nil, err
	}
	h.statusDB = db

	return db, nil
}

func (h *Handler) getRepositoryDB(ctx context.Context) (*staticdb.RepositoryDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.repositoryDB != nil {
		return h.repositoryDB, nil
	}

	db, err := staticdb.OpenRepositoryDB(ctx, h.Params.Bucket, h.repoDir, h.Repository)
	if err != nil {
		return nil, err
	}
	h.repositoryDB = db

	return db, nil
}

func (h *Handler) getLogDB(ctx context.Context) (*staticdb.LogDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.logDB != nil {
		return h.logDB, nil
	}

	db, err := staticdb.OpenLogDB(ctx, h.Params.Bucket, h.repoDir, h.Repository)
	if err != nil {
		return nil, err
	}
	h.logDB = db

	return db, nil
}

func (h *Handler) Start(ctx context.Context) {
	// initialize status DB
	statusDB, err := h.getStatusDB(ctx)
	if err != nil {
		h.cleanup()
		h.logger.Error("status DB initialization", "error", err.Error())
		return
	}

	numEvents, err := statusDB.CountEnvents(ctx)
	if err != nil {
		h.cleanup()
		h.logger.Error("status DB getting events count", "error", err.Error())
		return
	}

	h.logger.Info("status DB events count", "events", numEvents)

	err = statusDB.WalkEvents(ctx, func(event *eventv1.EventPayload) error {
		return h.QueueEvent(event, false)
	})
	if err != nil {
		h.cleanup()
		h.logger.Error("event queue", "error", err.Error())
		return
	}

	go func() {
		for !h.Stopped.Load() {
			select {
			case <-ctx.Done():
				h.Stopped.Store(true)
			case <-h.Queued:
				events := h.DequeueEvents()

				h.processEvents(events)

				if h.EventQueueLength() > 0 {
					h.EventQueueUpdate()
				}
			}
		}
		h.cleanup()
	}()
}

func (h *Handler) processEvents(events []*eventv1.EventPayload) {
	processContext := context.Background()

	for _, event := range events {
		manifest, err := v1.ParseManifest(bytes.NewReader(event.Payload))
		if err != nil {
			h.logger.Error("parse package manifest", "error", err.Error())
			continue
		}

		if event.Action == eventv1.Action_ACTION_PUT {
			switch manifest.Config.MediaType {
			case types.MediaType(orasfile.StaticFileConfigType):
				err := h.processFileManifest(processContext, manifest)
				if err != nil {
					h.logger.Error("process file manifest", "error", err.Error())
				}
			}
		} else if event.Action == eventv1.Action_ACTION_DELETE {
			switch manifest.Config.MediaType {
			case types.MediaType(orasfile.StaticFileConfigType):
				err := h.deleteFileManifest(processContext, manifest)
				if err != nil {
					h.logger.Error("delete package manifest", "error", err.Error())
				}
			}
		}

		if err := h.statusDB.RemoveEvent(processContext, event); err != nil {
			h.logger.Error("event remove", "error", err.Error())
		} else if err := h.statusDB.Sync(processContext); err != nil {
			h.logger.Error("event remove", "error", err.Error())
		}

		if h.Stopped.Load() {
			break
		}
	}
}
