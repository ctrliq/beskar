// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync"

	"go.ciq.dev/beskar/internal/plugins/yum/pkg/mirror"
	"go.ciq.dev/beskar/internal/plugins/yum/pkg/yumdb"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
	"golang.org/x/sync/semaphore"
)

func (h *Handler) repositorySync(ctx context.Context) (errFn error) {
	reposync := h.updateSyncing(true)
	queued := 0

	defer func() {
		if queued == 0 {
			_ = h.updateSyncing(false)
			if err := h.updateReposyncDatabase(dbCtx, reposync); err != nil {
				if errFn == nil {
					errFn = err
				}
			}
		}
	}()

	if err := h.updateReposyncDatabase(dbCtx, reposync); err != nil {
		_ = h.updateSyncing(false)
		return err
	}

	repoDB, err := h.getRepositoryDB(ctx)
	if err != nil {
		return err
	}
	defer repoDB.Close(false)

	syncer := mirror.NewSyncer(h.downloadDir(), h.getMirrorURLs())

	syncMutex := sync.Mutex{}

	syncedPackages := 0

	paths, totalPackages := syncer.DownloadPackages(ctx, func(id string) bool {
		has, _ := repoDB.HasPackageID(ctx, id)
		if has {
			syncedPackages++
		}
		return !has
	})

	sem := semaphore.NewWeighted(5)

	for path := range paths {
		queued++

		if err := sem.Acquire(ctx, 1); err != nil {
			break
		}

		go func(path string) {
			defer sem.Release(1)

			rc, err := syncer.FileReader(ctx, path)
			if err != nil {
				h.logger.Error("package download", "package", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "package %s download: %s", path, err)
				return
			}
			defer rc.Close()

			filename := filepath.Join(h.downloadDir(), filepath.Base(path))
			pkg, err := os.Create(filename)
			if err != nil {
				h.logger.Error("package create", "package", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "package %s download (create): %s", path, err)
				return
			}

			_, err = io.Copy(pkg, rc)
			closeErr := pkg.Close()

			if err != nil {
				_ = os.Remove(filename)
				h.logger.Error("package copy", "package", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "package %s download (copy): %s", path, err)
				return
			} else if closeErr != nil {
				_ = os.Remove(filename)
				h.logger.Error("package flush", "package", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "package %s download (close): %s", path, err)
				return
			}

			pusher, err := orasrpm.NewRPMPusher(filename, h.repository, h.params.NameOptions...)
			if err != nil {
				_ = os.Remove(filename)
				h.logger.Error("package push prepare", "package", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "package %s download (init push): %s", path, err)
				return
			}

			if err := oras.Push(pusher, h.params.RemoteOptions...); err != nil {
				_ = os.Remove(filename)
				h.logger.Error("package push", "package", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "package %s download (push): %s", path, err)
				return
			}

			syncMutex.Lock()
			syncedPackages++
			reposync, err := h.addSyncedPackageReposyncDatabase(ctx, syncedPackages, totalPackages)
			if err != nil {
				h.logger.Error("package push", "package", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "package %s download (push): %s", path, err)
			} else {
				h.setReposync(reposync)
			}
			syncMutex.Unlock()
		}(path)
	}

	if err := syncer.Err(); err != nil {
		return err
	}

	metaDB, err := h.getMetadataDB(ctx)
	if err != nil {
		return err
	}
	defer metaDB.Close(false)

	extras := make(map[string]string)

	err = metaDB.WalkExtraMetadata(ctx, func(em *yumdb.ExtraMetadata) error {
		extras[em.Type] = em.Checksum
		return nil
	})
	if err != nil {
		return err
	}

	extraMetadatas := syncer.DownloadExtraMetadata(ctx, func(dataType string, checksum string) bool {
		extraChecksum, ok := extras[dataType]
		if !ok {
			return true
		}
		return extraChecksum != checksum
	})

	for idx := range extraMetadatas {
		repomdData := syncer.RepomdData(idx)
		if repomdData == nil {
			continue
		}

		queued++

		if err := sem.Acquire(ctx, 1); err != nil {
			break
		}

		go func() {
			defer sem.Release(1)

			path := repomdData.Location.Href
			rc, err := syncer.FileReader(ctx, path)
			if err != nil {
				h.logger.Error("metadata download", "metadata", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "metadata %s download: %s", path, err)
				return
			}
			defer rc.Close()

			filename := filepath.Join(h.downloadDir(), filepath.Base(path))
			pkg, err := os.Create(filename)
			if err != nil {
				h.logger.Error("metadata create", "metadata", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "metadata %s download (create): %s", path, err)
				return
			}

			_, err = io.Copy(pkg, rc)
			closeErr := pkg.Close()

			if err != nil {
				_ = os.Remove(filename)
				h.logger.Error("metadata copy", "metadata", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "metadata %s download (copy): %s", path, err)
				return
			} else if closeErr != nil {
				_ = os.Remove(filename)
				h.logger.Error("metadata flush", "metadata", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "metadata %s download (close): %s", path, err)
				return
			}

			pusher, err := orasrpm.NewRPMExtraMetadataPusher(filename, h.repository, repomdData.Type, h.params.NameOptions...)
			if err != nil {
				_ = os.Remove(filename)
				h.logger.Error("metadata push prepare", "metadata", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "metadata %s download (init push): %s", path, err)
				return
			}

			if err := oras.Push(pusher, h.params.RemoteOptions...); err != nil {
				_ = os.Remove(filename)
				h.logger.Error("metadata push", "metadata", path, "error", err.Error())
				h.logDatabase(ctx, yumdb.LogError, "metadata %s download (push): %s", path, err)
				return
			}
		}()
	}

	return syncer.Err()
}
