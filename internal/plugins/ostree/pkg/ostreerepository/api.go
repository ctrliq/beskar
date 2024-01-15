// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostreerepository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/internal/plugins/ostree/pkg/libostree"
	"go.ciq.dev/beskar/pkg/orasostree"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/ostree/api/v1"
	"go.ciq.dev/beskar/pkg/utils"
	"golang.org/x/sync/errgroup"
)

func (h *Handler) CreateRepository(ctx context.Context, properties *apiv1.OSTreeRepositoryProperties) (err error) {
	h.logger.Debug("creating repository", "repository", h.Repository)
	// Validate request
	if len(properties.Remotes) == 0 {
		return ctl.Errf("at least one remote is required")
	}

	// Check if repo already exists
	if h.checkRepoExists(ctx) {
		return ctl.Err("repository already exists")
	}

	// Transition to provisioning state
	if err := h.setState(StateProvisioning); err != nil {
		return err
	}
	defer h.clearState()

	return h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *libostree.Repo) (bool, error) {
		// Add user provided remotes
		// We do not need to add beskar remote here
		for _, remote := range properties.Remotes {
			var opts []libostree.Option
			if remote.NoGPGVerify {
				opts = append(opts, libostree.NoGPGVerify())
			}
			if err := repo.AddRemote(remote.Name, remote.RemoteURL, opts...); err != nil {
				return false, ctl.Errf("adding remote to ostree repository %s: %s", remote.Name, err)
			}
		}

		if err := repo.RegenerateSummary(); err != nil {
			return false, ctl.Errf("regenerating summary for ostree repository %s: %s", h.repoDir, err)
		}

		return true, nil
	}, SkipPull())
}

// DeleteRepository deletes the repository from beskar and the local filesystem.
//
// This could lead to an invalid _state if the repository fails to completely deleting from beskar.
func (h *Handler) DeleteRepository(ctx context.Context) (err error) {
	// Transition to deleting state
	if err := h.setState(StateDeleting); err != nil {
		return err
	}

	// Check if repo already exists
	if !h.checkRepoExists(ctx) {
		defer h.clearState()
		return ctl.Err("repository does not exist")
	}

	go func() {
		defer func() {
			if err == nil {
				// stop the repo handler and trigger cleanup
				h.Stop()
			}
			h.clearState()
		}()
		h.logger.Debug("deleting repository")

		err := h.BeginLocalRepoTransaction(context.Background(), func(ctx context.Context, repo *libostree.Repo) (bool, error) {
			// Create a worker pool to deleting each file in the repository concurrently.
			// ctx will be cancelled on error, and the error will be returned.
			eg, ctx := errgroup.WithContext(ctx)
			eg.SetLimit(100)

			// Walk the directory tree, skipping directories and deleting each file.
			if err := filepath.WalkDir(h.repoDir, func(path string, d os.DirEntry, err error) error {
				// If there was an error with the file, return it.
				if err != nil {
					return fmt.Errorf("walking %s: %w", path, err)
				}
				// Skip directories.
				if d.IsDir() {
					return nil
				}
				// Skip the rest of the files if the context has been cancelled.
				if ctx.Err() != nil {
					// Skip remaining files because our context has been cancelled.
					// We could return the error here, but we want to exclusively handle that error in our call to eg.Wait().
					// This is because we would never be able to handle an error returned from the last job.
					return filepath.SkipAll
				}
				// Schedule deletion to run in a worker.
				eg.Go(func() error {
					// Delete the file from the repository
					filename := filepath.Base(path)
					h.logger.Debug("deleting file from beskar", "file", filename)
					digest := orasostree.MakeTag(filename)
					digestRef := filepath.Join(h.Repository, "file:"+digest)
					if err := h.DeleteManifest(digestRef); err != nil {
						h.logger.Error("deleting file from beskar", "error", err.Error())
					}

					return nil
				})

				return nil
			}); err != nil {
				return false, err
			}

			// We don't want to push any changes to beskar.
			return false, eg.Wait()
		})
		if err != nil {
			h.logger.Error("deleting repository", "error", err.Error())
		}
	}()

	return nil
}

func (h *Handler) AddRemote(ctx context.Context, remote *apiv1.OSTreeRemoteProperties) (err error) {
	// Transition to provisioning state
	if err := h.setState(StateProvisioning); err != nil {
		return err
	}
	defer h.clearState()

	if !h.checkRepoExists(ctx) {
		return ctl.Errf("repository does not exist")
	}

	return h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *libostree.Repo) (bool, error) {
		// Add user provided remote
		var opts []libostree.Option
		if remote.NoGPGVerify {
			opts = append(opts, libostree.NoGPGVerify())
		}
		if err := repo.AddRemote(remote.Name, remote.RemoteURL, opts...); err != nil {
			// No need to make error pretty, it is already pretty
			return false, err
		}

		return true, nil
	}, SkipPull())
}

func (h *Handler) SyncRepository(_ context.Context, properties *apiv1.OSTreeRepositorySyncRequest) (err error) {
	// Transition to syncing state
	if err := h.setState(StateSyncing); err != nil {
		return err
	}

	// Spin up pull worker
	go func() {
		h.logger.Debug("syncing repository")

		var err error
		defer func() {
			if err != nil {
				h.logger.Error("repository sync failed", "properties", properties, "error", err.Error())
				repoSync := *h.repoSync.Load()
				repoSync.SyncError = err.Error()
				h.setRepoSync(&repoSync)
			} else {
				h.logger.Debug("repository sync complete", "properties", properties)
			}
			h.clearState()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		err = h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *libostree.Repo) (bool, error) {
			// Pull the latest changes from the remote.
			opts := []libostree.Option{
				libostree.Depth(properties.Depth),
				libostree.Flags(libostree.Mirror | libostree.TrustedHTTP),
			}
			if len(properties.Refs) > 0 {
				opts = append(opts, libostree.Refs(properties.Refs...))
			}

			// pull remote content into local repo
			if err := repo.Pull(ctx, properties.Remote, opts...); err != nil {
				return false, ctl.Errf("pulling ostree repository: %s", err)
			}

			if err := repo.RegenerateSummary(); err != nil {
				return false, ctl.Errf("regenerating summary for ostree repository %s: %s", h.repoDir, err)
			}

			return true, nil
		})
	}()

	return nil
}

func (h *Handler) GetRepositorySyncStatus(_ context.Context) (syncStatus *apiv1.SyncStatus, err error) {
	repoSync := h.repoSync.Load()
	if repoSync == nil {
		return nil, ctl.Errf("repository sync status not available")
	}
	return &apiv1.SyncStatus{
		Syncing:   repoSync.Syncing,
		StartTime: utils.TimeToString(repoSync.StartTime),
		EndTime:   utils.TimeToString(repoSync.EndTime),
		SyncError: repoSync.SyncError,
	}, nil
}
