// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yumrepository

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/hashicorp/go-multierror"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/mirror"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
	"golang.org/x/sync/semaphore"
)

const syncMaxDownloads = 10

func (h *Handler) repositorySync(ctx context.Context) (errFn error) {
	reposync := h.updateSyncing(true)

	defer func() {
		reposync = h.updateSyncing(false)

		if errFn != nil {
			reposync.SyncError = errFn.Error()
		} else {
			reposync.SyncError = ""
		}
		if err := h.updateReposyncDatabase(dbCtx, reposync); err != nil {
			if errFn == nil {
				errFn = err
			} else {
				h.logger.Error("reposync database update failed", "error", err.Error())
			}
		}
		h.syncArtifactsMutex.Lock()
		for k, v := range h.syncArtifacts {
			delete(h.syncArtifacts, k)
			close(v)
		}
		h.syncArtifactsMutex.Unlock()
	}()

	if err := h.updateReposyncDatabase(dbCtx, reposync); err != nil {
		return err
	}

	packageMutex := sync.Mutex{}
	packages := new(multierror.Group)

	sem := semaphore.NewWeighted(syncMaxDownloads)
	semAcquire := func() error {
		semCtx, cancel := context.WithTimeout(ctx, time.Minute)
		defer cancel()

		return sem.Acquire(semCtx, 1)
	}

	repoDB, err := h.getRepositoryDB(dbCtx)
	if err != nil {
		return err
	}
	defer repoDB.Close(false)

	dbPackages := make(map[string]struct{})

	err = repoDB.WalkPackages(dbCtx, func(pkg *yumdb.RepositoryPackage) error {
		dbPackages[pkg.ID] = struct{}{}
		return nil
	})
	if err != nil {
		return err
	}

	syncer := mirror.NewSyncer(h.downloadDir(), h.getMirrorURLs(), mirror.WithKeyring(h.getKeyring()))

	syncedPackages := 0
	updateMetadata := false

	paths, totalPackages := syncer.DownloadPackages(ctx, func(id string) bool {
		_, has := dbPackages[id]
		if has {
			delete(dbPackages, id)
			syncedPackages++
		}
		return !has
	})

	for path := range paths {
		updateMetadata = true

		if err := semAcquire(); err != nil {
			merr := packages.Wait()
			h.logger.Error("package download semaphore timeout", "package", path, "error", err.Error())
			return multierror.Append(merr, err)
		}

		path := path

		packages.Go(func() error {
			fullPath := filepath.Join(h.downloadDir(), filepath.Base(path))

			defer func() {
				sem.Release(1)
				_ = os.Remove(fullPath)
			}()

			rc, err := syncer.FileReader(ctx, path)
			if err != nil {
				h.logger.Error("package download", "package", path, "error", err.Error())
				return fmt.Errorf("package %s download: %w", path, err)
			}
			defer rc.Close()

			if err := copyTo(rc, fullPath); err != nil {
				h.logger.Error("package copy", "package", path, "error", err.Error())
				return fmt.Errorf("package %s copy: %w", path, err)
			}

			pusher, err := orasrpm.NewRPMPusher(fullPath, h.Repository, h.Params.NameOptions...)
			if err != nil {
				h.logger.Error("package push prepare", "package", path, "error", err.Error())
				return fmt.Errorf("package %s push prepare: %w", path, err)
			}

			img, err := pusher.Image()
			if err != nil {
				h.logger.Error("package push prepare", "package", path, "error", err.Error())
				return fmt.Errorf("package %s push prepare: %w", path, err)
			}
			manifest, err := img.Manifest()
			if err != nil {
				h.logger.Error("package oras manifest", "package", path, "error", err.Error())
				return fmt.Errorf("package %s oras manifest: %w", path, err)
			}
			rpmName := manifest.Layers[0].Annotations[imagespec.AnnotationTitle]
			if rpmName == "" {
				h.logger.Error("package name from oras layer", "package", path, "error", err.Error())
				return fmt.Errorf("package %s name from oras layer", path)
			}

			pkgTag := ""
			if pt, ok := pusher.Reference().(name.Tag); ok {
				pkgTag = pt.TagStr()
			}
			if pkgTag == "" {
				h.logger.Error("package tag empty", "package", path)
				return fmt.Errorf("package %s tag: empty", path)
			}

			pkg, err := repoDB.GetPackageByTag(dbCtx, pkgTag)
			if err != nil {
				h.logger.Error("package database by tag", "package", path, "tag", pkgTag, "error", err.Error())
				return fmt.Errorf("package %s database by tag: %w", path, err)
			} else if pkg.ID != "" {
				// ensure to not delete an overridden package
				packageMutex.Lock()
				delete(dbPackages, pkg.ID)
				packageMutex.Unlock()
			}

			errCh := make(chan error, 1)

			h.syncArtifactsMutex.Lock()
			h.syncArtifacts[rpmName] = errCh
			h.syncArtifactsMutex.Unlock()

			if err := oras.Push(pusher, h.Params.RemoteOptions...); err != nil {
				h.logger.Error("package push", "package", path, "error", err.Error())
				errCh <- fmt.Errorf("package %s push: %w", path, err)
			}

			pkgCtx, cancel := context.WithTimeout(ctx, time.Minute)

			var pkgErr error

			select {
			case <-pkgCtx.Done():
				if errors.Is(pkgCtx.Err(), context.Canceled) {
					pkgErr = fmt.Errorf("package %s processing timeout", path)
				} else {
					pkgErr = fmt.Errorf("package %s processing interrupted", path)
				}
			case pkgErr = <-errCh:
				if pkgErr == nil {
					packageMutex.Lock()
					syncedPackages++
					reposync, err := h.addSyncedPackageReposyncDatabase(dbCtx, syncedPackages, totalPackages)
					if err != nil {
						h.logger.Error("reposync status update", "package", path, "error", err.Error())
						h.logDatabase(dbCtx, yumdb.LogError, "reposync status update for %s: %s", path, err)
					} else {
						h.setReposync(reposync)
					}
					packageMutex.Unlock()
				}
			}

			h.syncArtifactsMutex.Lock()
			close(errCh)
			delete(h.syncArtifacts, rpmName)
			h.syncArtifactsMutex.Unlock()

			cancel()

			return pkgErr
		})
	}

	if err := syncer.Err(); err != nil {
		return err
	} else if err := packages.Wait(); err != nil {
		return err
	} else if !updateMetadata && len(dbPackages) == 0 {
		return nil
	}

	for pkgID := range dbPackages {
		updateMetadata = true
		pkgID := pkgID

		if err := semAcquire(); err != nil {
			merr := packages.Wait()
			h.logger.Error("package removal semaphore timeout", "pkgid", pkgID, "error", err.Error())
			return multierror.Append(merr, err)
		}

		packages.Go(func() error {
			defer sem.Release(1)

			pkg, err := repoDB.GetPackage(dbCtx, pkgID)
			if err != nil {
				h.logger.Error("package database by id", "pkgid", pkgID, "error", err.Error())
				return fmt.Errorf("package id %s database: %w", pkgID, err)
			}

			arch := pkg.Architecture
			if pkg.SourceRPM == "" {
				arch = "src"
			}
			rpmName := fmt.Sprintf("%s-%s-%s.%s.rpm", pkg.Name, pkg.Version, pkg.Release, arch)

			tagRef := filepath.Join(h.Repository, "packages:"+pkg.Tag)

			digest, err := h.GetManifestDigest(tagRef)
			if err != nil {
				return werror.Wrap(gcode.ErrInternal, err)
			}

			digestRef := filepath.Join(h.Repository, "packages@"+digest)

			errCh := make(chan error, 1)

			h.syncArtifactsMutex.Lock()
			h.syncArtifacts[rpmName] = errCh
			h.syncArtifactsMutex.Unlock()

			if err := h.DeleteManifest(digestRef); err != nil {
				return werror.Wrap(gcode.ErrInternal, err)
			}

			pkgCtx, cancel := context.WithTimeout(ctx, time.Minute)

			var pkgErr error

			select {
			case <-pkgCtx.Done():
				if errors.Is(pkgCtx.Err(), context.Canceled) {
					pkgErr = fmt.Errorf("package %s removal processing timeout", rpmName)
				} else {
					pkgErr = fmt.Errorf("package %s removal processing interrupted", rpmName)
				}
			case pkgErr = <-errCh:
			}

			h.syncArtifactsMutex.Lock()
			close(errCh)
			delete(h.syncArtifacts, rpmName)
			h.syncArtifactsMutex.Unlock()

			cancel()

			return pkgErr
		})
	}

	if err := syncer.Err(); err != nil {
		return err
	} else if err := packages.Wait(); err != nil {
		return err
	} else if !updateMetadata {
		return nil
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
		return fmt.Errorf("repomd.xml copy: %w", err)
	}

	repomdXML, err := orasrpm.NewGenericRPMMetadata(repomdXMLPath, orasrpm.RepomdXMLLayerType, nil)
	if err != nil {
		h.logger.Error("metadata push initialization", "metadata", repomdXMLPath, "error", err.Error())
		return fmt.Errorf("metadata %s push initialization: %w", repomdXMLPath, err)
	}

	metadataLayers = append(metadataLayers, orasrpm.NewRPMMetadataLayer(repomdXML))

	if sigReader := syncer.RepomdXMLSignature(); sigReader != nil {
		repomdXMLSignaturePath := filepath.Join(metadataDir, "repomd.xml.asc")

		if err := copyTo(sigReader, repomdXMLSignaturePath); err != nil {
			h.logger.Error("metadata copy", "metadata", repomdXMLSignaturePath, "error", err.Error())
			return fmt.Errorf("metadata %s copy: %w", repomdXMLSignaturePath, err)
		}

		repomdXMLSignature, err := orasrpm.NewGenericRPMMetadata(
			repomdXMLSignaturePath,
			orasrpm.RepomdXMLSignatureLayerType,
			nil,
		)
		if err != nil {
			h.logger.Error("metadata push initialization", "metadata", repomdXMLSignaturePath, "error", err.Error())
			return fmt.Errorf("metadata %s push initialization: %w", repomdXMLSignaturePath, err)
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
			return fmt.Errorf("metadata %s download: %w", path, err)
		}

		fullPath := filepath.Join(metadataDir, filepath.Base(path))

		err = copyTo(rc, fullPath)
		_ = rc.Close()
		if err != nil {
			h.logger.Error("metadata copy", "metadata", path, "error", err.Error())
			return fmt.Errorf("metadata %s copy: %w", path, err)
		}

		mediatype := orasrpm.GetRepomdDataLayerType(repomdData.Type)
		annotations := map[string]string{
			imagespec.AnnotationTitle: filepath.Base(fullPath),
		}
		metadataLayer, err := orasrpm.NewGenericRPMMetadata(fullPath, mediatype, annotations)
		if err != nil {
			h.logger.Error("metadata push initialization", "metadata", fullPath, "error", err.Error())
			return fmt.Errorf("metadata %s push initialization: %w", fullPath, err)
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
