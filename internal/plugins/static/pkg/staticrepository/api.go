// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package staticrepository

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"go.ciq.dev/beskar/internal/plugins/static/pkg/staticdb"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/static/api/v1"
)

func (h *Handler) DeleteRepository(_ context.Context) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	}

	return werror.Wrap(gcode.ErrNotImplemented, errors.New("repository delete not supported yet"))
}

func (h *Handler) ListRepositoryLogs(ctx context.Context, _ *apiv1.Page) (logs []apiv1.RepositoryLog, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getLogDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	err = db.WalkLogs(ctx, func(log *staticdb.Log) error {
		logs = append(logs, apiv1.RepositoryLog{
			Level:   log.Level,
			Message: log.Message,
			Date:    timeToString(log.Date),
		})
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return logs, nil
}

func (h *Handler) RemoveRepositoryFile(ctx context.Context, tag string) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	file, err := db.GetFileByTag(ctx, tag)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	} else if file.Tag == "" {
		return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("file with tag %s not found", tag))
	}

	tagRef := filepath.Join(h.Repository, "files:"+file.Tag)

	digest, err := h.GetManifestDigest(tagRef)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	digestRef := filepath.Join(h.Repository, "files@"+digest)

	if err := h.DeleteManifest(digestRef); err != nil {
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

	file, err := db.GetFileByTag(ctx, tag)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	} else if file.Tag == "" {
		return nil, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("file with tag %s not found", tag))
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

	file, err := db.GetFileByName(ctx, name)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	} else if file.Tag == "" {
		return nil, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("file with name %s not found", name))
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
		UploadTime: timeToString(pkg.UploadTime),
		Size:       pkg.Size,
	}
}

const timeFormat = time.DateTime + " MST"

func timeToString(t int64) string {
	if t == 0 {
		return ""
	}
	return time.Unix(t, 0).Format(timeFormat)
}
