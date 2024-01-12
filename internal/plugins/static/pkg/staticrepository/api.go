// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package staticrepository

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"go.ciq.dev/beskar/pkg/utils"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"github.com/hashicorp/go-multierror"
	"go.ciq.dev/beskar/internal/pkg/sqlite"
	"go.ciq.dev/beskar/internal/plugins/static/pkg/staticdb"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/static/api/v1"
	"golang.org/x/sync/semaphore"
)

func (h *Handler) DeleteRepository(ctx context.Context, deleteFiles bool) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
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

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	if !deleteFiles {
		count, err := db.CountFiles(ctx)
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
		err = db.WalkFiles(ctx, func(file *staticdb.RepositoryFile) error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			deleteFile.Go(func() error {
				defer sem.Release(1)
				return h.removeRepositoryFile(ctx, file)
			})
			return nil
		})
		if err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
		if err := deleteFile.Wait().ErrorOrNil(); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	return nil
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

	err = db.WalkLogs(ctx, func(log *staticdb.Log) error {
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

func (h *Handler) removeRepositoryFile(ctx context.Context, file *staticdb.RepositoryFile) error {
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

func (h *Handler) RemoveRepositoryFile(ctx context.Context, tag string) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if h.delete.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	file, err := db.GetFileByTag(ctx, tag)
	if err != nil {
		if errors.Is(err, sqlite.ErrNoEntryFound) {
			return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("file with tag %s not found", tag))
		}
		return werror.Wrap(gcode.ErrInternal, err)
	} else if err := h.removeRepositoryFile(ctx, file); err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	return nil
}

func (h *Handler) GetRepositoryFileByTag(ctx context.Context, tag string) (repositoryFile *apiv1.RepositoryFile, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	file, err := db.GetFileByTag(ctx, tag)
	if err != nil {
		if errors.Is(err, sqlite.ErrNoEntryFound) {
			return nil, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("file with tag %s not found", tag))
		}
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return toRepositoryFileAPI(file), nil
}

func (h *Handler) GetRepositoryFileByName(ctx context.Context, name string) (repositoryFile *apiv1.RepositoryFile, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	file, err := db.GetFileByName(ctx, name)
	if err != nil {
		if errors.Is(err, sqlite.ErrNoEntryFound) {
			return nil, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("file with name %s not found", name))
		}
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return toRepositoryFileAPI(file), nil
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

	err = db.WalkFiles(ctx, func(file *staticdb.RepositoryFile) error {
		repositoryFiles = append(repositoryFiles, toRepositoryFileAPI(file))
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return repositoryFiles, nil
}

func toRepositoryFileAPI(pkg *staticdb.RepositoryFile) *apiv1.RepositoryFile {
	return &apiv1.RepositoryFile{
		Tag:        pkg.Tag,
		ID:         pkg.ID,
		Name:       pkg.Name,
		UploadTime: utils.TimeToString(pkg.UploadTime),
		Size:       pkg.Size,
	}
}
