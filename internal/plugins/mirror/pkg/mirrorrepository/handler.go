// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"bytes"
	"context"
	"encoding/gob"
	"log/slog"
	"net/url"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"go.ciq.dev/beskar/internal/pkg/repository"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrordb"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
)

type Handler struct {
	*repository.RepoHandler

	repoDir string

	logger *slog.Logger

	dbMutex      sync.Mutex
	repositoryDB *mirrordb.RepositoryDB
	statusDB     *mirrordb.StatusDB
	logDB        *mirrordb.LogDB

	syncCh  chan chan error
	sync    atomic.Pointer[mirrordb.Sync]
	syncing atomic.Bool

	propertyMutex sync.RWMutex
	created       bool
	mirror        bool
	mirrorURLs    []*url.URL

	delete atomic.Bool
}

func NewHandler(logger *slog.Logger, repoHandler *repository.RepoHandler) *Handler {
	return &Handler{
		RepoHandler: repoHandler,
		repoDir:     filepath.Join(repoHandler.Params.Dir, repoHandler.Repository),
		logger:      logger,
		syncCh:      make(chan chan error, 1),
	}
}

func (h *Handler) QueueEvent(event *eventv1.EventPayload, _ bool) error {
	h.logger.Info("queued event", "digest", event.Digest)

	h.EnqueueEvent(event)

	return nil
}

func (h *Handler) getRepositoryDB(ctx context.Context) (*mirrordb.RepositoryDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.repositoryDB != nil {
		return h.repositoryDB, nil
	}

	db, err := mirrordb.OpenRepositoryDB(ctx, h.Params.Bucket, h.repoDir, h.Repository)
	if err != nil {
		return nil, err
	}
	h.repositoryDB = db

	return db, nil
}

func (h *Handler) getStatusDB(ctx context.Context) (*mirrordb.StatusDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.statusDB != nil {
		return h.statusDB, nil
	}

	db, err := mirrordb.OpenStatusDB(ctx, h.Params.Bucket, h.repoDir, h.Repository)
	if err != nil {
		return nil, err
	}
	h.statusDB = db

	return db, nil
}

func (h *Handler) getLogDB(ctx context.Context) (*mirrordb.LogDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.logDB != nil {
		return h.logDB, nil
	}

	db, err := mirrordb.OpenLogDB(ctx, h.Params.Bucket, h.repoDir, h.Repository)
	if err != nil {
		return nil, err
	}
	h.logDB = db

	return db, nil
}

func (h *Handler) cleanup() {
	h.logger.Debug("repository cleanup", "repository", h.Repository)

	h.dbMutex.Lock()

	if h.logDB != nil {
		if err := h.logDB.Close(true); err != nil {
			h.logger.Error("log database close", "error", err.Error())
		}
		if h.delete.Load() {
			if err := h.logDB.Delete(context.Background()); err != nil {
				h.logger.Error("log database delete", "error", err.Error())
			}
		}
	}
	if h.statusDB != nil {
		if err := h.statusDB.Close(true); err != nil {
			h.logger.Error("status database close", "error", err.Error())
		}
		if h.delete.Load() {
			if err := h.statusDB.Delete(context.Background()); err != nil {
				h.logger.Error("status database delete", "error", err.Error())
			}
		}
	}
	if h.repositoryDB != nil {
		if err := h.repositoryDB.Close(true); err != nil {
			h.logger.Error("repository database close", "error", err.Error())
		}
		if h.delete.Load() {
			if err := h.repositoryDB.Delete(context.Background()); err != nil {
				h.logger.Error("repository database delete", "error", err.Error())
			}
		}
	}

	h.dbMutex.Unlock()

	close(h.Queued)
	close(h.syncCh)
	h.Params.Remove(h.Repository)
}

func (h *Handler) initProperties(ctx context.Context) error {
	statusDB, err := h.getStatusDB(ctx)
	if err != nil {
		return err
	}

	properties, err := statusDB.GetProperties(ctx)
	if err != nil {
		return err
	}
	h.setMirror(properties.Mirror)
	h.setCreated(properties.Created)

	if len(properties.MirrorURLs) > 0 {
		var mirrorURLs []string

		decoder := gob.NewDecoder(bytes.NewReader(properties.MirrorURLs))
		if err := decoder.Decode(&mirrorURLs); err != nil {
			return err
		}

		if err := h.setMirrorURLs(mirrorURLs); err != nil {
			return err
		}
	}

	sync, err := statusDB.GetSync(ctx)
	if err != nil {
		return err
	}
	h.setSync(sync)

	return nil
}

func (h *Handler) setMirrorURLs(urls []string) error {
	var err error

	mirrorURLs := make([]*url.URL, len(urls))

	for i, u := range urls {
		mirrorURLs[i], err = url.Parse(u)
		if err != nil {
			return err
		}
	}

	h.propertyMutex.Lock()
	h.mirrorURLs = mirrorURLs
	h.propertyMutex.Unlock()

	return nil
}

func (h *Handler) getMirrorURLs() []*url.URL {
	h.propertyMutex.RLock()
	defer h.propertyMutex.RUnlock()

	return h.mirrorURLs
}

func (h *Handler) setMirror(b bool) {
	h.propertyMutex.Lock()
	h.mirror = b
	h.propertyMutex.Unlock()
}

func (h *Handler) setCreated(created bool) {
	h.propertyMutex.Lock()
	h.created = created
	h.propertyMutex.Unlock()
}

func (h *Handler) isCreated() bool {
	h.propertyMutex.RLock()
	defer h.propertyMutex.RUnlock()

	return h.created
}

func (h *Handler) getMirror() bool {
	h.propertyMutex.RLock()
	defer h.propertyMutex.RUnlock()

	return h.mirror
}

func (h *Handler) getSync() *mirrordb.Sync {
	return h.sync.Load()
}

func (h *Handler) setSync(sync *mirrordb.Sync) {
	s := *sync
	h.sync.Store(&s)
}

func (h *Handler) updateSyncing(syncing bool) *mirrordb.Sync {
	sync := *h.getSync()
	previousSyncing := sync.Syncing
	sync.Syncing = syncing
	h.syncing.Store(syncing)
	if syncing && !previousSyncing {
		sync.StartTime = time.Now().UTC().Unix()
		sync.SyncError = ""
	} else if !syncing && previousSyncing {
		sync.EndTime = time.Now().UTC().Unix()
	}
	h.sync.Store(&sync)
	return h.sync.Load()
}

func (h *Handler) Start(ctx context.Context) {
	// initialize status DB
	statusDB, err := h.getStatusDB(ctx)
	if err != nil {
		h.cleanup()
		h.logger.Error("status DB initialization", "error", err.Error())
		return
	}

	if err := h.initProperties(ctx); err != nil {
		h.cleanup()
		h.logger.Error("status DB properties initialization", "error", err.Error())
		return
	}

	sync := h.getSync()

	h.propertyMutex.RLock()
	mirrorCount := len(h.mirrorURLs)
	h.propertyMutex.RUnlock()

	if sync.Syncing && mirrorCount == 0 {
		sync.Syncing = false
	}

	numEvents, err := statusDB.CountEvents(ctx)
	if err != nil {
		h.cleanup()
		h.logger.Error("status DB getting events count", "error", err.Error())
		return
	}
	if h.getMirror() {
		if !sync.Syncing && numEvents > 0 {
			sync.Syncing = true
		} else if numEvents == 0 && sync.Syncing {
			sync.Syncing = false
		}
	}

	h.logger.Info("status DB events count", "events", numEvents)

	h.updateSyncing(sync.Syncing)

	var lastIndex *eventv1.EventPayload

	err = statusDB.WalkEvents(ctx, func(event *eventv1.EventPayload) error {
		lastIndex = event
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
			case waitErrCh, more := <-h.syncCh:
				if more {
					go func() {
						err := h.repositorySync(ctx)
						if err != nil {
							h.logger.Error("reposistory sync", "error", err.Error())
						}
						if waitErrCh != nil {
							waitErrCh <- err
							close(waitErrCh)
						}
					}()
				}
			case <-h.Queued:
				events := h.DequeueEvents()

				if !h.isCreated() {
					h.setCreated(true)
					if err := statusDB.SetCreatedProperty(dbCtx); err != nil {
						h.logger.Error("status DB set created property", "error", err.Error())
					}
				}

				h.processEvents(events)

				// all remaining events in database have been processed
				// start the repository sync
				if lastIndex != nil && events[len(events)-1].Digest == lastIndex.Digest {
					// false means that the sync operation won't wait for the
					// sync to complete
					h.syncCh <- nil
					lastIndex = nil
				}

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
		// Don't care about events, just process them.
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
