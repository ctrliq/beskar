// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumrepository

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/mirror"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
)

const syncMaxDownloads = 10

func (h *Handler) repositorySync(ctx context.Context, sem *mirror.Semaphore) (errFn error) {
	reposync := h.updateSyncing(true)

	defer func() {
		if err := sem.Acquire(ctx, syncMaxDownloads, time.Minute); err == nil {
			sem.Release(syncMaxDownloads)
		}

		reposync = h.updateSyncing(false)
		if errFn != nil {
			reposync.SyncError = errFn.Error()
		} else {
			reposync.SyncError = ""
		}
		if err := h.updateReposyncDatabase(dbCtx, reposync); err != nil {
			if errFn == nil {
				errFn = err
			}
		}
	}()

	if err := h.updateReposyncDatabase(dbCtx, reposync); err != nil {
		_ = h.updateSyncing(false)
		return err
	}

	repoDB, err := h.getRepositoryDB(dbCtx)
	if err != nil {
		return err
	}
	defer repoDB.Close(false)

	syncer := mirror.NewSyncer(h.downloadDir(), h.getMirrorURLs(), mirror.WithKeyring(h.getKeyring()))

	syncMutex := sync.Mutex{}

	syncedPackages := 0

	paths, totalPackages := syncer.DownloadPackages(ctx, func(id string) bool {
		has, _ := repoDB.HasPackageID(ctx, id)
		if has {
			syncedPackages++
		}
		return !has
	})

	for path := range paths {
		if err := sem.Acquire(ctx, 1, time.Minute); err != nil {
			return err
		}

		go func(path string) {
			rc, err := syncer.FileReader(ctx, path)
			if err != nil {
				h.logger.Error("package download", "package", path, "error", err.Error())
				h.logDatabase(dbCtx, yumdb.LogError, "package %s retrieval: %s", path, err)
				return
			}
			defer rc.Close()

			fullPath := filepath.Join(h.downloadDir(), filepath.Base(path))
			if err := copyTo(rc, fullPath); err != nil {
				_ = os.Remove(fullPath)
				h.logger.Error("package copy", "package", path, "error", err.Error())
				h.logDatabase(dbCtx, yumdb.LogError, "package %s copy: %s", path, err)
				return
			}

			pusher, err := orasrpm.NewRPMPusher(fullPath, h.Repository, h.Params.NameOptions...)
			if err != nil {
				_ = os.Remove(fullPath)
				h.logger.Error("package push prepare", "package", path, "error", err.Error())
				h.logDatabase(dbCtx, yumdb.LogError, "package %s push initialization: %s", path, err)
				return
			}

			if err := oras.Push(pusher, h.Params.RemoteOptions...); err != nil {
				_ = os.Remove(fullPath)
				h.logger.Error("package push", "package", path, "error", err.Error())
				h.logDatabase(dbCtx, yumdb.LogError, "package %s push: %s", path, err)
				return
			}

			syncMutex.Lock()
			syncedPackages++
			reposync, err := h.addSyncedPackageReposyncDatabase(dbCtx, syncedPackages, totalPackages)
			if err != nil {
				h.logger.Error("reposync status update", "package", path, "error", err.Error())
				h.logDatabase(dbCtx, yumdb.LogError, "reposync status update for %s: %s", path, err)
			} else {
				h.setReposync(reposync)
			}
			syncMutex.Unlock()
		}(path)
	}

	if err := syncer.Err(); err != nil {
		return err
	}

	pushRef, err := name.ParseReference(
		filepath.Join(h.Repository, "repodata:"+RepomdXMLTag),
		h.Params.NameOptions...,
	)
	if err != nil {
		return err
	}

	metadataDir := filepath.Join(h.downloadDir(), "metadata")
	if err := os.MkdirAll(metadataDir, 0o700); err != nil {
		return err
	}
	defer os.RemoveAll(metadataDir)

	metadataCount := 1

	metadatas := syncer.DownloadMetadata(ctx, func(dataType string, checksum string) bool {
		metadataCount++
		return true
	})

	metadataLayers := make([]oras.Layer, 0, metadataCount)

	repomdXMLPath := filepath.Join(metadataDir, "repomd.xml")

	if err := copyTo(syncer.RepomdXML(), repomdXMLPath); err != nil {
		h.logger.Error("repomd.xml copy", "metadata", repomdXMLPath, "error", err.Error())
		h.logDatabase(dbCtx, yumdb.LogError, "%s metadata copy: %s", repomdXMLPath, err)
		return err
	}

	repomdXML, err := orasrpm.NewGenericRPMMetadata(repomdXMLPath, orasrpm.RepomdXMLLayerType, nil)
	if err != nil {
		h.logger.Error("metadata push initialization", "metadata", repomdXMLPath, "error", err.Error())
		h.logDatabase(dbCtx, yumdb.LogError, "%s metadata push initialization: %s", repomdXMLPath, err)
		return err
	}

	metadataLayers = append(metadataLayers, orasrpm.NewRPMMetadataLayer(repomdXML))

	if sigReader := syncer.RepomdXMLSignature(); sigReader != nil {
		repomdXMLSignaturePath := filepath.Join(metadataDir, "repomd.xml.asc")

		if err := copyTo(sigReader, repomdXMLSignaturePath); err != nil {
			h.logger.Error("metadata copy", "metadata", repomdXMLSignaturePath, "error", err.Error())
			h.logDatabase(dbCtx, yumdb.LogError, "%s metadata copy: %s", repomdXMLSignaturePath, err)
			return err
		}

		repomdXMLSignature, err := orasrpm.NewGenericRPMMetadata(
			repomdXMLSignaturePath,
			orasrpm.RepomdXMLSignatureLayerType,
			nil,
		)
		if err != nil {
			h.logger.Error("metadata push initialization", "metadata", repomdXMLSignaturePath, "error", err.Error())
			h.logDatabase(dbCtx, yumdb.LogError, "%s metadata push initialization: %s", repomdXMLSignaturePath, err)
			return err
		}

		metadataLayers = append(metadataLayers, orasrpm.NewRPMMetadataLayer(repomdXMLSignature))
	}

	for idx := range metadatas {
		repomdData := syncer.RepomdData(idx)
		if repomdData == nil {
			continue
		}

		path := repomdData.Location.Href
		rc, err := syncer.FileReader(ctx, path)
		if err != nil {
			h.logger.Error("metadata download", "metadata", path, "error", err.Error())
			h.logDatabase(dbCtx, yumdb.LogError, "metadata %s retrieval: %s", path, err)
			return err
		}

		fullPath := filepath.Join(metadataDir, filepath.Base(path))

		err = copyTo(rc, fullPath)
		_ = rc.Close()
		if err != nil {
			h.logger.Error("metadata copy", "metadata", path, "error", err.Error())
			h.logDatabase(dbCtx, yumdb.LogError, "metadata %s copy: %s", path, err)
			return err
		}

		mediatype := orasrpm.GetRepomdDataLayerType(repomdData.Type)
		annotations := map[string]string{
			imagespec.AnnotationTitle: filepath.Base(fullPath),
		}
		metadataLayer, err := orasrpm.NewGenericRPMMetadata(fullPath, mediatype, annotations)
		if err != nil {
			h.logger.Error("metadata push initialization", "metadata", fullPath, "error", err.Error())
			h.logDatabase(dbCtx, yumdb.LogError, "%s metadata push initialization: %s", fullPath, err)
		}
		metadataLayers = append(metadataLayers, orasrpm.NewRPMMetadataLayer(metadataLayer))
	}

	if err := syncer.Err(); err != nil {
		return err
	}

	return oras.Push(
		orasrpm.NewRPMMetadataPusher(pushRef, orasrpm.RepomdConfigType, metadataLayers...),
		h.Params.RemoteOptions...,
	)
}

func copyTo(src io.Reader, dest string) error {
	pkg, err := os.Create(dest)
	if err != nil {
		return err
	}

	_, err = io.Copy(pkg, src)
	closeErr := pkg.Close()
	if err != nil {
		return err
	} else if closeErr != nil {
		return closeErr
	}

	return nil
}
