// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
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
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/hashicorp/go-multierror"
	"go.ciq.dev/beskar/internal/pkg/sqlite"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/yum/api/v1"
	"golang.org/x/sync/semaphore"
)

var dbCtx = context.Background()

const timeFormat = time.DateTime + " MST"

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

func (h *Handler) DeleteRepository(ctx context.Context, deletePackages bool) (err error) {
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

	if !deletePackages {
		count, err := repoDB.CountPackages(ctx)
		if err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		} else if count > 0 {
			return werror.Wrap(gcode.ErrFailedPrecondition, fmt.Errorf("repository %s has %d packages associated with it", h.Repository, count))
		}
	} else {
		deletePackage := new(multierror.Group)
		// maximum parallel package deletion
		sem := semaphore.NewWeighted(100)

		// delete all packages associated with this repo
		err = repoDB.WalkPackages(ctx, func(pkg *yumdb.RepositoryPackage) error {
			if err := sem.Acquire(ctx, 1); err != nil {
				return err
			}
			deletePackage.Go(func() error {
				defer sem.Release(1)
				return h.removePackageFromBeskar(ctx, pkg)
			})
			return nil
		})
		if err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		} else if err := deletePackage.Wait().ErrorOrNil(); err != nil {
			return werror.Wrap(gcode.ErrInternal, err)
		}
	}

	// delete all metadata associated with this repo
	metaDB, err := h.getMetadataDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	deleteMetadata := new(multierror.Group)

	deleteMetadata.Go(func() error {
		// nil meta means repomd.xml
		if err := h.removeMetadataFromBeskar(ctx, nil); err != nil {
			// repomd.xml was not found, it could be because
			// no packages have been uploaded yet, it's fine
			// to ignore it
			var transportErr *transport.Error
			if errors.As(err, &transportErr) && transportErr.StatusCode == 404 {
				return nil
			}
		}
		return nil
	})

	err = metaDB.WalkExtraMetadata(ctx, func(meta *yumdb.ExtraMetadata) error {
		deleteMetadata.Go(func() error {
			return h.removeMetadataFromBeskar(ctx, meta)
		})
		return nil
	})
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	} else if err := deleteMetadata.Wait().ErrorOrNil(); err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
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
	defer db.Close(false)

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

	if len(propertiesDB.MirrorURLs) > 0 {
		decoder := gob.NewDecoder(bytes.NewReader(propertiesDB.MirrorURLs))
		if err := decoder.Decode(&properties.MirrorURLs); err != nil {
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
	} else if len(h.getMirrorURLs()) == 0 {
		return werror.Wrap(gcode.ErrFailedPrecondition, errors.New("repository doesn't have mirror URLs setup"))
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
	defer db.Close(false)

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

func (h *Handler) removeMetadataFromBeskar(ctx context.Context, meta *yumdb.ExtraMetadata) error {
	tag := RepomdXMLTag
	if meta != nil {
		tag = meta.Type
	}

	tagRef := filepath.Join(h.Repository, "repodata:"+tag)

	digest, err := h.GetManifestDigest(tagRef)
	if err != nil {
		return err
	}
	digestRef := filepath.Join(h.Repository, "repodata@"+digest)

	if meta != nil {
		fileName := meta.Filename

		errCh, waitSync := h.SyncArtifact(ctx, fileName, time.Minute)

		if err := h.DeleteManifest(digestRef); err != nil {
			errCh <- err
		}

		if err := waitSync(); err != nil {
			return fmt.Errorf("metadata %s removal processing error: %w", tag, err)
		}
	} else {
		if err := h.DeleteManifest(digestRef); err != nil {
			return fmt.Errorf("metadata %s removal processing error: %w", tag, err)
		}
	}

	return nil
}

func (h *Handler) removePackageFromBeskar(ctx context.Context, pkg *yumdb.RepositoryPackage) error {
	tagRef := filepath.Join(h.Repository, "packages:"+pkg.Tag)

	digest, err := h.GetManifestDigest(tagRef)
	if err != nil {
		return err
	}

	rpmName := pkg.RPMName()

	errCh, waitSync := h.SyncArtifact(ctx, rpmName, time.Minute)

	digestRef := filepath.Join(h.Repository, "packages@"+digest)

	if err := h.DeleteManifest(digestRef); err != nil {
		errCh <- err
	}

	if err := waitSync(); err != nil {
		return fmt.Errorf("package %s removal processing error: %w", rpmName, err)
	}

	return nil
}

func (h *Handler) RemoveRepositoryPackage(ctx context.Context, id string) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if h.getMirror() {
		return werror.Wrap(gcode.ErrFailedPrecondition, fmt.Errorf("could not delete package for mirror repository"))
	} else if h.delete.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	pkg, err := db.GetPackage(ctx, id)
	if err != nil {
		if errors.Is(err, sqlite.ErrNoEntryFound) {
			return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("package with id %s not found", id))
		}
		return werror.Wrap(gcode.ErrInternal, err)
	}

	if err := h.removePackageFromBeskar(ctx, pkg); err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}

	return nil
}

func (h *Handler) RemoveRepositoryPackageByTag(ctx context.Context, tag string) (err error) {
	if !h.Started() {
		return werror.Wrap(gcode.ErrUnavailable, err)
	} else if h.getMirror() {
		return werror.Wrap(gcode.ErrFailedPrecondition, fmt.Errorf("could not delete package for mirror repository"))
	} else if h.delete.Load() {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository %s is being deleted", h.Repository))
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	pkg, err := db.GetPackageByTag(ctx, tag)
	if err != nil {
		if errors.Is(err, sqlite.ErrNoEntryFound) {
			return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("package with tag %s not found", tag))
		}
		return werror.Wrap(gcode.ErrInternal, err)
	}

	if err := h.removePackageFromBeskar(ctx, pkg); err != nil {
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
	defer db.Close(false)

	pkg, err := db.GetPackage(ctx, id)
	if err != nil {
		if errors.Is(err, sqlite.ErrNoEntryFound) {
			return nil, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("package with id %s not found", id))
		}
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}

	return toRepositoryPackageAPI(pkg), nil
}

func (h *Handler) GetRepositoryPackageByTag(ctx context.Context, tag string) (repositoryPackage *apiv1.RepositoryPackage, err error) {
	if !h.Started() {
		return nil, werror.Wrap(gcode.ErrUnavailable, err)
	}

	db, err := h.getRepositoryDB(ctx)
	if err != nil {
		return nil, werror.Wrap(gcode.ErrInternal, err)
	}
	defer db.Close(false)

	pkg, err := db.GetPackageByTag(ctx, tag)
	if err != nil {
		if errors.Is(err, sqlite.ErrNoEntryFound) {
			return nil, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("package with tag %s not found", tag))
		}
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
	defer db.Close(false)

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
