// SPDX-FileCopyrightText: Copyright (c) 2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package mirrorrepository

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"github.com/hashicorp/go-multierror"
	"go.ciq.dev/beskar/internal/plugins/mirror/pkg/mirrordb"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/mirror/api/v1"
	"go.ciq.dev/beskar/pkg/utils"
	"golang.org/x/sync/semaphore"
)

var dbCtx = context.Background()

func (h *Handler) CreateRepository(ctx context.Context, properties *apiv1.RepositoryProperties) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if properties == nil {
		return werror.Wrap(gcode.ErrInvalidArgument, fmt.Errorf("properties can't be nil"))
	}

	db, err := h.getStatusDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	propertiesDB, err := db.GetProperties(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	} else if propertiesDB.Created {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s already exists", h.Repository))
	}

	propertiesDB.Created = true

	if properties.Mirror != nil {
		propertiesDB.Mirror = *properties.Mirror
	}
	h.setMirror(propertiesDB.Mirror)

	if properties.MirrorConfigs != nil {
		buf := new(bytes.Buffer)
		decoder := gob.NewEncoder(buf)
		if err := decoder.Encode(properties.MirrorConfigs); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
		propertiesDB.MirrorConfigs = buf.Bytes()
		if err := h.setMirrorConfigs(properties.MirrorConfigs); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	if properties.WebConfig != nil {
		buf := new(bytes.Buffer)
		decoder := gob.NewEncoder(buf)
		if err := decoder.Encode(properties.WebConfig); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
		propertiesDB.WebConfig = buf.Bytes()
		if err := h.setWebConfig(properties.WebConfig); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	if err := db.UpdateProperties(dbCtx, propertiesDB); err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	return db.Sync(dbCtx)
}

func (h *Handler) DeleteRepository(ctx context.Context, deleteFiles bool) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if h.syncing.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, errors.New("a repository sync is running"))
	} else if h.delete.Swap(true) {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	}
	defer func() {
		h.SyncArtifactReset()

		if err == nil {
			// stop the repo handler and trigger cleanup
			h.Stop()
		} else {
			h.delete.Store(false)
		}
	}()

	db, err := h.getStatusDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	propertiesDB, err := db.GetProperties(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	} else if !propertiesDB.Created {
		return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("repository %s not found", h.Repository))
	}

	// check if there are any pkgs associated with this repo
	repoDB, err := h.getRepositoryDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	if !deleteFiles {
		count, err := repoDB.CountFiles(ctx)
		if err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		} else if count > 0 {
			return werror.Wrap(gcode.ErrFailedPrecondition, fmt.Errorf("repository %s has %d files associated with it", h.Repository, count))
		}
	} else {
		deleteFile := new(multierror.Group)
		// maximum parallel file deletion
		sem := semaphore.NewWeighted(100)

		// delete all files associated with this repo
		err = repoDB.WalkFiles(ctx, func(pkg *mirrordb.RepositoryFile) error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			deleteFile.Go(func() error {
				defer sem.Release(1)
				return h.removeRepositoryFile(ctx, pkg)
			})
			return nil
		})
		if err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		} else if err := deleteFile.Wait().ErrorOrNil(); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	return nil
}

func (h *Handler) UpdateRepository(ctx context.Context, properties *apiv1.RepositoryProperties) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if properties == nil {
		return werror.Wrap(gcode.ErrInvalidArgument, fmt.Errorf("properties can't be nil"))
	} else if h.delete.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	}

	db, err := h.getStatusDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	propertiesDB, err := db.GetProperties(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	} else if !propertiesDB.Created {
		return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("repository is not created"))
	}

	if properties.Mirror != nil {
		propertiesDB.Mirror = *properties.Mirror
	}
	h.setMirror(propertiesDB.Mirror)

	if properties.MirrorConfigs != nil {
		buf := new(bytes.Buffer)
		decoder := gob.NewEncoder(buf)
		if err := decoder.Encode(properties.MirrorConfigs); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
		propertiesDB.MirrorConfigs = buf.Bytes()
		if err := h.setMirrorConfigs(properties.MirrorConfigs); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	if properties.WebConfig != nil {
		buf := new(bytes.Buffer)
		decoder := gob.NewEncoder(buf)
		if err := decoder.Encode(properties.WebConfig); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
		propertiesDB.WebConfig = buf.Bytes()
		if err := h.setWebConfig(properties.WebConfig); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	if err := db.UpdateProperties(dbCtx, propertiesDB); err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	return db.Sync(dbCtx)
}

func (h *Handler) GetRepository(ctx context.Context) (properties *apiv1.RepositoryProperties, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getStatusDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	propertiesDB, err := db.GetProperties(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	} else if !propertiesDB.Created {
		return nil, werror.Wrap(gcode.ErrNotFound, errors.New("repository not found"))
	}

	properties = &apiv1.RepositoryProperties{
		Mirror: &propertiesDB.Mirror,
	}

	if len(propertiesDB.MirrorConfigs) > 0 {
		decoder := gob.NewDecoder(bytes.NewReader(propertiesDB.MirrorConfigs))
		if err := decoder.Decode(&properties.MirrorConfigs); err != nil {
			return nil, werror.Wrap(gcode.ErrInternal, err)
		}
	}

	if len(propertiesDB.WebConfig) > 0 {
		decoder := gob.NewDecoder(bytes.NewReader(propertiesDB.WebConfig))
		if err := decoder.Decode(&properties.WebConfig); err != nil {
			return nil, werror.Wrap(gcode.ErrInternal, err)
		}
	}

	return properties, nil
}

func (h *Handler) SyncRepository(_ context.Context, wait bool) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if !h.getMirror() {
		return werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository not setup as a mirror"))
	} else if len(h.getMirrorConfigs()) == 0 {
		return werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository doesn't have mirror configurations setup"))
	} else if h.delete.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	} else if h.syncing.Swap(true) {
		return werror.Wrap(gcode.ErrAlreadyExists, errors.New("a repository sync is already running"))
	}

	var waitErrCh chan error

	if wait {
		waitErrCh = make(chan error, 1)
	}

	select {
	case h.syncCh <- waitErrCh:
		if waitErrCh != nil {
			if err := <-waitErrCh; err != nil {
				return werror.Wrap(gcode.ErrInternal, fmt.Errorf("synchronization failed: %w", err))
			}
		}
	default:
		return werror.Wrap(gcode.ErrUnavailable, errors.New("something goes wrong"))
	}

	return nil
}

func (h *Handler) SyncRepositoryWithConfig(_ context.Context, mirrorConfigs []apiv1.MirrorConfig, webConfig *apiv1.WebConfig, wait bool) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if !h.getMirror() {
		return werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository not setup as a mirror"))
	}

	// Set mirror configs if supplied.
	if mirrorConfigs != nil {
		if err := h.setMirrorConfigs(mirrorConfigs); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	// Set web config if supplied.
	if webConfig != nil {
		if err := h.setWebConfig(webConfig); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	if h.delete.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	} else if h.syncing.Swap(true) {
		return werror.Wrap(gcode.ErrAlreadyExists, errors.New("a repository sync is already running"))
	}

	var waitErrCh chan error

	if wait {
		waitErrCh = make(chan error, 1)
	}

	select {
	case h.syncCh <- waitErrCh:
		if waitErrCh != nil {
			if err := <-waitErrCh; err != nil {
				return werror.Wrap(gcode.ErrInternal, fmt.Errorf("synchronization failed: %w", err))
			}
		}
	default:
		return werror.Wrap(gcode.ErrUnavailable, errors.New("something goes wrong"))
	}

	return nil
}

func (h *Handler) GenerateRepository(_ context.Context) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if !h.getMirror() {
		return werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository not setup as a mirror"))
	} else if len(h.getMirrorConfigs()) == 0 {
		return werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository doesn't have mirror configurations setup"))
	} else if h.delete.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	} else if h.syncing.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, errors.New("a repository sync is running"))
	}

	if err := h.GenerateIndexes(); err != nil {
		h.logger.Error("failed to generate indexes", "error", err)
		return werror.Wrap(gcode.ErrUnavailable, errors.New("unable to generate indexes"))
	}

	return nil
}

func (h *Handler) GetRepositorySyncPlan(_ context.Context) (syncPlan *apiv1.RepositorySyncPlan, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	} else if !h.getMirror() {
		return nil, werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository not setup as a mirror"))
	} else if len(h.getMirrorConfigs()) == 0 {
		return nil, werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository doesn't have mirror configurations setup"))
	} else if h.delete.Load() {
		return nil, werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	} else if h.syncing.Load() {
		return nil, werror.Wrap(gcode.ErrAlreadyExists, errors.New("a repository sync is running"))
	}

	plan, err := h.getSyncPlan()
	if err != nil {
		return nil, werror.Wrap(gcode.ErrUnavailable, errors.New("unable to generate sync plan"))
	}

	return plan, nil
}

func (h *Handler) GetRepositorySyncStatus(context.Context) (syncStatus *apiv1.SyncStatus, err error) {
	sync := h.getSync()
	return &apiv1.SyncStatus{
		Syncing:   sync.Syncing,
		StartTime: utils.TimeToString(sync.StartTime),
		EndTime:   utils.TimeToString(sync.EndTime),
		SyncError: sync.SyncError,
	}, nil
}

func (h *Handler) ListRepositoryLogs(ctx context.Context, _ *apiv1.Page) (logs []apiv1.RepositoryLog, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getLogDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	err = db.WalkLogs(ctx, func(log *mirrordb.Log) error {
		logs = append(logs, apiv1.RepositoryLog{
			Level:   log.Level,
			Message: log.Message,
			Date:    utils.TimeToString(log.Date),
		})
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return logs, nil
}

func (h *Handler) ListRepositoryFiles(ctx context.Context, _ *apiv1.Page) (repositoryFiles []*apiv1.RepositoryFile, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	err = db.WalkFiles(ctx, func(file *mirrordb.RepositoryFile) error {
		repositoryFiles = append(repositoryFiles, toRepositoryFileAPI(file))
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return repositoryFiles, nil
}

func (h *Handler) ListRepositorySymlinks(ctx context.Context, _ *apiv1.Page) (repositoryFiles []*apiv1.RepositoryFile, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	err = db.WalkSymlinks(ctx, func(file *mirrordb.RepositoryFile) error {
		repositoryFiles = append(repositoryFiles, toRepositoryFileAPI(file))
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return repositoryFiles, nil
}

func (h *Handler) GetRepositoryFile(ctx context.Context, name string) (repositoryFile *apiv1.RepositoryFile, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	fileDB, err := db.GetFileByName(ctx, name)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return toRepositoryFileAPI(fileDB), nil
}

func (h *Handler) GetRepositoryFileByReferenceRaw(ctx context.Context, reference string) (repositoryFile *mirrordb.RepositoryFile, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	fileDB, err := db.GetFileByReference(ctx, reference)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return fileDB, nil
}

func (h *Handler) GetRepositoryFileByReference(ctx context.Context, reference string) (repositoryFile *apiv1.RepositoryFile, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	fileDB, err := db.GetFileByReference(ctx, reference)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return toRepositoryFileAPI(fileDB), nil
}

func (h *Handler) GetRepositoryFileCount(ctx context.Context) (count int, err error) {
	if !h.Started() {
		return 0, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return 0, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	count, err = db.CountFiles(ctx)
	if err != nil {
		return 0, werror.Wrap(gcode.ErrInternal, err)
	}

	return count, nil
}

func (h *Handler) DeleteRepositoryFile(ctx context.Context, file string) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	err = db.DeleteFileByName(ctx, file)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	return nil
}

func (h *Handler) removeRepositoryFile(ctx context.Context, file *mirrordb.RepositoryFile) error {
	tagRef := filepath.Join(h.Repository, "files:"+file.Tag)

	digest, err := h.GetManifestDigest(tagRef)
	if err != nil {
		return err
	}

	errCh, waitSync := h.SyncArtifact(ctx, file.Name, time.Minute)

	digestRef := filepath.Join(h.Repository, "files@"+digest)

	if err := h.DeleteManifest(digestRef); err != nil {
		errCh <- err
	}

	if err := waitSync(); err != nil {
		return fmt.Errorf("file %s removal processing error: %w", file.Name, err)
	}

	return nil
}

func toRepositoryFileAPI(file *mirrordb.RepositoryFile) *apiv1.RepositoryFile {
	return &apiv1.RepositoryFile{
		Tag:          file.Tag,
		Name:         file.Name,
		Reference:    file.Reference,
		Parent:       file.Parent,
		Link:         file.Link,
		ModifiedTime: utils.TimeToString(file.ModifiedTime),
		Mode:         file.Mode,
		Size:         file.Size,
		ConfigID:     file.ConfigID,
	}
}

func toRepositoryFileDB(file *apiv1.RepositoryFile) *mirrordb.RepositoryFile {
	return &mirrordb.RepositoryFile{
		Tag:          file.Tag,
		Name:         file.Name,
		Reference:    file.Reference,
		Parent:       file.Parent,
		Link:         file.Link,
		ModifiedTime: utils.StringToTime(file.ModifiedTime),
		Mode:         file.Mode,
		Size:         file.Size,
		ConfigID:     file.ConfigID,
	}
}
