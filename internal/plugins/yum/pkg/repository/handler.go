// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	eventv1 "go.ciq.dev/beskar/pkg/api/event/v1"
	"go.ciq.dev/beskar/pkg/orasrpm"
	"gocloud.dev/blob"
	"golang.org/x/crypto/openpgp"       //nolint:staticcheck
	"golang.org/x/crypto/openpgp/armor" //nolint:staticcheck
	"golang.org/x/exp/slog"
)

type HandlerParams struct {
	Dir           string
	Bucket        *blob.Bucket
	RemoteOptions []remote.Option
	NameOptions   []name.Option
	Remove        func(string)
}

type Handler struct {
	cancel context.CancelFunc

	repository string
	repoDir    string
	params     *HandlerParams

	queue      []*eventv1.EventPayload
	queueMutex sync.Mutex
	queued     chan struct{}

	stopped atomic.Bool

	logger *slog.Logger

	dbMutex      sync.Mutex
	repositoryDB *yumdb.RepositoryDB
	metadataDB   *yumdb.MetadataDB
	statusDB     *yumdb.StatusDB
	logDB        *yumdb.LogDB

	syncCh   chan struct{}
	reposync atomic.Pointer[yumdb.Reposync]
	syncing  atomic.Bool

	propertyMutex sync.RWMutex
	mirror        bool
	keyring       openpgp.KeyRing
	mirrorURLs    []*url.URL
}

func NewHandler(logger *slog.Logger, repository string, params *HandlerParams) *Handler {
	ctx, cancel := context.WithCancel(context.Background())

	handler := &Handler{
		cancel:     cancel,
		repository: repository,
		repoDir:    filepath.Join(params.Dir, repository),
		params:     params,
		logger:     logger,
		queued:     make(chan struct{}, 1),
		syncCh:     make(chan struct{}, 1),
	}

	handler.start(ctx)

	return handler
}

func (h *Handler) downloadDir() string {
	return filepath.Join(h.repoDir, "downloads")
}

func (h *Handler) QueueEvent(event *eventv1.EventPayload, store bool) error {
	ctx := context.Background()

	if h.getMirror() && !h.getReposync().Syncing {
		return fmt.Errorf("")
	}

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

	h.queueMutex.Lock()
	h.queue = append(h.queue, event)
	h.queueMutex.Unlock()

	h.notifyQueue()

	return nil
}

func (h *Handler) notifyQueue() {
	select {
	case h.queued <- struct{}{}:
	default:
	}
}

func (h *Handler) getRepositoryDB(ctx context.Context) (*yumdb.RepositoryDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.repositoryDB != nil {
		return h.repositoryDB, nil
	}

	db, err := yumdb.OpenRepositoryDB(ctx, h.params.Bucket, h.repoDir, h.repository)
	if err != nil {
		return nil, err
	}
	h.repositoryDB = db

	return db, nil
}

func (h *Handler) getMetadataDB(ctx context.Context) (*yumdb.MetadataDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.metadataDB != nil {
		return h.metadataDB, nil
	}

	db, err := yumdb.OpenMetadataDB(ctx, h.params.Bucket, h.repoDir, h.repository)
	if err != nil {
		return nil, err
	}
	h.metadataDB = db

	return db, nil
}

func (h *Handler) getStatusDB(ctx context.Context) (*yumdb.StatusDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.statusDB != nil {
		return h.statusDB, nil
	}

	db, err := yumdb.OpenStatusDB(ctx, h.params.Bucket, h.repoDir, h.repository)
	if err != nil {
		return nil, err
	}
	h.statusDB = db

	return db, nil
}

func (h *Handler) getLogDB(ctx context.Context) (*yumdb.LogDB, error) {
	h.dbMutex.Lock()
	defer h.dbMutex.Unlock()

	if h.logDB != nil {
		return h.logDB, nil
	}

	db, err := yumdb.OpenLogDB(ctx, h.params.Bucket, h.repoDir, h.repository)
	if err != nil {
		return nil, err
	}
	h.logDB = db

	return db, nil
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
	if h.metadataDB != nil {
		h.metadataDB.Close(true)
	}

	h.dbMutex.Unlock()

	close(h.queued)
	close(h.syncCh)
	h.params.Remove(h.repository)
	_ = os.RemoveAll(h.repoDir)
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
	if len(properties.GPGKey) > 0 {
		if err := h.setKeyring(properties.GPGKey); err != nil {
			return err
		}
	}

	reposync, err := statusDB.GetReposync(ctx)
	if err != nil {
		return err
	}

	h.setReposync(reposync)

	if reposync.Syncing {
		if len(properties.MirrorURLs) > 0 {
			h.syncing.Store(true)
			h.syncCh <- struct{}{}
		} else {
			reposync.Syncing = false
		}
	}

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

func (h *Handler) getMirror() bool {
	h.propertyMutex.RLock()
	defer h.propertyMutex.RUnlock()

	return h.mirror
}

func (h *Handler) setKeyring(key []byte) error {
	p, err := armor.Decode(bytes.NewReader(key))
	if err != nil {
		return err
	}

	keyring, err := openpgp.ReadKeyRing(p.Body)
	if err != nil {
		return err
	}

	h.propertyMutex.Lock()
	h.keyring = keyring
	h.propertyMutex.Unlock()

	return nil
}

func (h *Handler) getKeyring() openpgp.KeyRing {
	h.propertyMutex.RLock()
	defer h.propertyMutex.RUnlock()

	return h.keyring
}

func (h *Handler) getReposync() *yumdb.Reposync {
	return h.reposync.Load()
}

func (h *Handler) setReposync(reposync *yumdb.Reposync) {
	rs := *reposync
	h.reposync.Store(&rs)
}

func (h *Handler) updateSyncing(b bool) *yumdb.Reposync {
	reposync := *h.getReposync()
	reposync.Syncing = b
	h.syncing.Store(b)
	if b {
		reposync.LastSyncTime = time.Now().UTC().Unix()
	}
	h.reposync.Store(&reposync)
	return &reposync
}

func (h *Handler) start(ctx context.Context) {
	if err := os.MkdirAll(h.downloadDir(), 0o700); err != nil {
		h.cleanup()
		h.logger.Error("create repository download dir", "error", err.Error())
		return
	}

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

	err = statusDB.WalkEvents(ctx, func(event *eventv1.EventPayload) error {
		return h.QueueEvent(event, false)
	})
	if err != nil {
		h.cleanup()
		h.logger.Error("event queue", "error", err.Error())
		return
	}

	dbCtx := context.Background()

	go func() {
		for !h.stopped.Load() {
			select {
			case <-ctx.Done():
				h.stopped.Store(true)
			case <-h.syncCh:
				go func() {
					err := h.repositorySync(dbCtx)
					if err != nil {
						h.logger.Error("reposistory sync", "error", err.Error())
					}
				}()
			case <-h.queued:
				h.queueMutex.Lock()
				events := h.queue
				h.queue = nil
				h.queueMutex.Unlock()
				h.processEvents(events)
				if len(h.queue) > 0 {
					h.notifyQueue()
				} else if h.getReposync().Syncing {
					reposync := h.updateSyncing(false)
					if dbErr := h.updateReposyncDatabase(dbCtx, reposync); dbErr != nil {
						h.logger.Error("reposync database update", "error", dbErr.Error())
					}
				}
			}
		}
		h.cleanup()
	}()
}

func (h *Handler) Started() bool {
	if h.stopped.Load() {
		<-h.queued
		return false
	}
	return true
}

func (h *Handler) Stop() {
	h.stopped.Store(true)
	h.cancel()
	<-h.queued
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
			case types.MediaType(orasrpm.RPMConfigType):
				err := h.processPackageManifest(processContext, manifest)
				if err != nil {
					h.logger.Error("process package manifest", "error", err.Error())
				}
			case types.MediaType(orasrpm.RepomdDataConfigType):
				err := h.processMetadataManifest(processContext, manifest)
				if err != nil {
					h.logger.Error("process metadata manifest", "error", err.Error())
				}
			}
		} else if event.Action == eventv1.Action_ACTION_DELETE {
			switch manifest.Config.MediaType {
			case types.MediaType(orasrpm.RPMConfigType):
				err := h.deletePackageManifest(processContext, manifest)
				if err != nil {
					h.logger.Error("delete package manifest", "error", err.Error())
				}
			case types.MediaType(orasrpm.RepomdDataConfigType):
				err := h.deleteMetadataManifest(processContext, manifest)
				if err != nil {
					h.logger.Error("delete metadata manifest", "error", err.Error())
				}
			}
		}

		if err := h.statusDB.RemoveEvent(processContext, event.Digest); err != nil {
			h.logger.Error("event remove", "error", err.Error())
		} else if err := h.statusDB.Sync(processContext); err != nil {
			h.logger.Error("event remove", "error", err.Error())
		}

		if h.stopped.Load() {
			break
		}
	}

	err := h.generateAndPushMetadata(processContext)
	if err != nil {
		h.logger.Error("generate/push metadata", "error", err.Error())
	}
}
