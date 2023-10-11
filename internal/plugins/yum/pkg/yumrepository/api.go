// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumrepository

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
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s already exists", h.Repository))
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
	} else if !propertiesDB.Created {
		return nil, werror.Wrap(gcode.ErrNotFound, errors.New("repository not found"))
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

	select {
	case h.syncCh <- struct{}{}:
	default:
		return werror.Wrap(gcode.ErrUnavailable, errors.New("something goes wrong"))
	}

	return nil
}

func (h *Handler) GetRepositorySyncStatus(context.Context) (syncStatus *apiv1.SyncStatus, err error) {
	reposync := h.getReposync()
	return &apiv1.SyncStatus{
		Syncing:        reposync.Syncing,
		StartTime:      timeToString(reposync.StartTime),
		EndTime:        timeToString(reposync.EndTime),
		TotalPackages:  reposync.TotalPackages,
		SyncedPackages: reposync.SyncedPackages,
		SyncError:      reposync.SyncError,
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
			Date:    timeToString(log.Date),
		})
		return nil
	})
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return logs, nil
}

func (h *Handler) RemoveRepositoryPackage(ctx context.Context, id string) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if h.getMirror() {
		return werror.Wrap(gcode.ErrFailedPrecondition, fmt.Errorf("could not delete package for mirror repository"))
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	pkg, err := db.GetPackage(ctx, id)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	} else if pkg.Tag == "" {
		return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("package with ID %s not found", id))
	}

	tagRef := filepath.Join(h.Repository, "packages:"+pkg.Tag)

	digest, err := h.GetManifestDigest(tagRef)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	digestRef := filepath.Join(h.Repository, "packages@"+digest)

	if err := h.DeleteManifest(digestRef); err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	return nil
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
		UploadTime:   timeToString(pkg.UploadTime),
		BuildTime:    timeToString(pkg.BuildTime),
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

func timeToString(t int64) string {
	if t == 0 {
		return ""
	}
	return time.Unix(t, 0).Format(timeFormat)
}
