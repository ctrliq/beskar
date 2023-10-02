// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"time"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/yum/api/v1"
)

var dbCtx = context.Background()

const timeFormat = time.DateTime + " MST"

func (h *Handler) CreateRepository(ctx context.Context, properties *apiv1.RepositoryProperties) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getStatusDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	propertiesDB, err := db.GetProperties(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	} else if propertiesDB.Created {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s already exists", h.repository))
	}

	propertiesDB.Created = true

	if properties.Mirror != nil {
		propertiesDB.Mirror = *properties.Mirror
	}
	h.setMirror(propertiesDB.Mirror)

	if properties.GPGKey != nil {
		propertiesDB.GPGKey = properties.GPGKey
		if err := h.setKeyring(propertiesDB.GPGKey); err != nil {
			return werror.Wrap(gcode.ErrInvalidArgument, errors.New("bad gpg key"))
		}
	}
	if properties.MirrorURLs != nil {
		buf := new(bytes.Buffer)
		decoder := gob.NewEncoder(buf)
		if err := decoder.Encode(properties.MirrorURLs); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
		propertiesDB.MirrorURLs = buf.Bytes()
		if err := h.setMirrorURLs(properties.MirrorURLs); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	if err := db.UpdateProperties(dbCtx, propertiesDB); err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	return db.Sync(dbCtx)
}

func (h *Handler) DeleteRepository(_ context.Context) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	}

	return werror.Wrap(gcode.ErrNotImplemented, errors.New("repository delete not supported yet"))
}

func (h *Handler) UpdateRepository(ctx context.Context, properties *apiv1.RepositoryProperties) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getStatusDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	propertiesDB, err := db.GetProperties(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	if properties.Mirror != nil {
		propertiesDB.Mirror = *properties.Mirror
	}
	h.setMirror(propertiesDB.Mirror)

	if properties.GPGKey != nil {
		propertiesDB.GPGKey = properties.GPGKey
		if err := h.setKeyring(propertiesDB.GPGKey); err != nil {
			return werror.Wrap(gcode.ErrInvalidArgument, errors.New("bad gpg key"))
		}
	}
	if properties.MirrorURLs != nil {
		buf := new(bytes.Buffer)
		decoder := gob.NewEncoder(buf)
		if err := decoder.Encode(properties.MirrorURLs); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
		propertiesDB.MirrorURLs = buf.Bytes()
		if err := h.setMirrorURLs(properties.MirrorURLs); err != nil {
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

	propertiesDB, err := db.GetProperties(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	properties = &apiv1.RepositoryProperties{
		Mirror: &propertiesDB.Mirror,
		GPGKey: propertiesDB.GPGKey,
	}

	decoder := gob.NewDecoder(bytes.NewReader(propertiesDB.MirrorURLs))
	if err := decoder.Decode(&properties.MirrorURLs); err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return properties, nil
}

func (h *Handler) SyncRepository(context.Context) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if !h.getMirror() {
		return werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository not setup as a mirror"))
	} else if len(h.getMirrorURLs()) == 0 {
		return werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository doesn't have mirror URLs setup"))
	}

	if h.syncing.Swap(true) {
		return werror.Wrap(gcode.ErrAlreadyExists, errors.New("a repository sync is already running"))
	}

	h.syncCh <- struct{}{}

	return nil
}

func (h *Handler) GetRepositorySyncStatus(context.Context) (syncStatus *apiv1.SyncStatus, err error) {
	reposync := h.getReposync()
	return &apiv1.SyncStatus{
		Syncing:        reposync.Syncing,
		LastSyncTime:   time.Unix(reposync.LastSyncTime, 0).Format(timeFormat),
		TotalPackages:  reposync.TotalPackages,
		SyncedPackages: reposync.SyncedPackages,
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

	err = db.WalkLogs(ctx, func(log *yumdb.Log) error {
		logs = append(logs, apiv1.RepositoryLog{
			Level:   log.Level,
			Message: log.Message,
			Date:    time.Unix(log.Date, 0).Format(timeFormat),
		})
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return logs, nil
}

func (h *Handler) RemoveRepositoryPackage(_ context.Context, _ string) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	}

	return werror.Wrap(gcode.ErrNotImplemented, errors.New("package removal not supported yet"))
}

func (h *Handler) GetRepositoryPackage(ctx context.Context, id string) (repositoryPackage *apiv1.RepositoryPackage, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	pkg, err := db.GetPackage(ctx, id)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return toRepositoryPackageAPI(pkg), nil
}

func (h *Handler) ListRepositoryPackages(ctx context.Context, _ *apiv1.Page) (repositoryPackages []*apiv1.RepositoryPackage, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	err = db.WalkPackages(ctx, func(pkg *yumdb.RepositoryPackage) error {
		repositoryPackages = append(repositoryPackages, toRepositoryPackageAPI(pkg))
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return repositoryPackages, nil
}

func toRepositoryPackageAPI(pkg *yumdb.RepositoryPackage) *apiv1.RepositoryPackage {
	return &apiv1.RepositoryPackage{
		Tag:          pkg.Tag,
		ID:           pkg.ID,
		Name:         pkg.Name,
		UploadTime:   time.Unix(pkg.UploadTime, 0).Format(timeFormat),
		BuildTime:    time.Unix(pkg.BuildTime, 0).Format(timeFormat),
		Size:         pkg.Size,
		Architecture: pkg.Architecture,
		SourceRPM:    pkg.SourceRPM,
		Version:      pkg.Version,
		Release:      pkg.Release,
		Groups:       pkg.Groups,
		License:      pkg.License,
		Vendor:       pkg.Vendor,
		Summary:      pkg.Summary,
		Description:  pkg.Description,
		Verified:     pkg.Verified,
		GPGSignature: pkg.GPGSignature,
	}
}
