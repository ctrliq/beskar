// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostreerepository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/RussellLuo/kun/pkg/werror"
	"github.com/RussellLuo/kun/pkg/werror/gcode"
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
		return werror.Wrap(gcode.ErrInvalidArgument, fmt.Errorf("remotes are required"))
	}

	// Check if repo already exists
	if h.checkRepoExists(ctx) {
		return werror.Wrap(gcode.ErrAlreadyExists, fmt.Errorf("repository already exists"))
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
				return false, werror.Wrap(gcode.ErrInternal, fmt.Errorf("adding remote to ostree repository %s: %w", remote.Name, err))
			}
		}

		if err := repo.RegenerateSummary(); err != nil {
			return false, werror.Wrap(gcode.ErrInternal, fmt.Errorf("regenerating summary for ostree repository %s: %w", h.repoDir, err))
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
		return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("repository does not exist"))
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
			eg.SetLimit(h.Params.Sync.GetMaxWorkerCount())

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

func (h *Handler) ListRepositoryRefs(ctx context.Context) (refs []apiv1.OSTreeRef, err error) {
	// Transition to provisioning state
	if err := h.setState(StateProvisioning); err != nil {
		return nil, err
	}
	defer h.clearState()

	if !h.checkRepoExists(ctx) {
		return nil, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("repository does not exist"))
	}

	err = h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *libostree.Repo) (bool, error) {
		// Get the refs from the local repo
		loRefs, err := repo.ListRefsExt(libostree.ListRefsExtFlagsNone)
		if err != nil {
			return false, werror.Wrap(gcode.ErrInternal, fmt.Errorf("listing refs from ostree repository: %w", err))
		}

		// Convert the refs to the API type
		for _, loRef := range loRefs {
			if loRef.Name == "" || loRef.Checksum == "" {
				return false, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("encountered ref with empty name or checksum"))
			}
			refs = append(refs, apiv1.OSTreeRef{
				Name:   loRef.Name,
				Commit: loRef.Checksum,
			})
		}

		return false, nil
	})

	return refs, err
}

func (h *Handler) AddRemote(ctx context.Context, remote *apiv1.OSTreeRemoteProperties) (err error) {
	// Transition to provisioning state
	if err := h.setState(StateProvisioning); err != nil {
		return err
	}
	defer h.clearState()

	if !h.checkRepoExists(ctx) {
		return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("repository does not exist"))
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

func (h *Handler) UpdateRemote(ctx context.Context, remoteName string, remote *apiv1.OSTreeRemoteProperties) (err error) {
	// Transition to provisioning state
	if err := h.setState(StateProvisioning); err != nil {
		return err
	}
	defer h.clearState()

	if !h.checkRepoExists(ctx) {
		return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("repository does not exist"))
	}

	return h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *libostree.Repo) (bool, error) {
		// Delete user provided remote
		if err := repo.DeleteRemote(remoteName); err != nil {
			// No need to make error pretty, it is already pretty
			return false, err
		}

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

func (h *Handler) DeleteRemote(ctx context.Context, remoteName string) (err error) {
	// Transition to provisioning state
	if err := h.setState(StateProvisioning); err != nil {
		return err
	}
	defer h.clearState()

	if !h.checkRepoExists(ctx) {
		return werror.Wrap(gcode.ErrNotFound, fmt.Errorf("repository does not exist"))
	}

	return h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *libostree.Repo) (bool, error) {
		// Delete user provided remote
		if err := repo.DeleteRemote(remoteName); err != nil {
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

		ctx, cancel := context.WithTimeout(context.Background(), properties.Timeout.AsDuration())
		defer cancel()

		err = h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *libostree.Repo) (commit bool, transactionFnErr error) {
			// Pull the latest changes from the remote.
			opts := h.standardPullOptions(libostree.Depth(properties.Depth))
			if len(properties.Refs) > 0 {
				opts = append(opts, libostree.Refs(properties.Refs...))
			}

			remoteName := properties.Remote
			if properties.EphemeralRemote != nil {
				remoteName = properties.EphemeralRemote.Name

				var opts []libostree.Option
				if properties.EphemeralRemote.NoGPGVerify {
					opts = append(opts, libostree.NoGPGVerify())
				}

				if err := repo.AddRemote(properties.EphemeralRemote.Name, properties.EphemeralRemote.RemoteURL, opts...); err != nil {
					// No need to make error pretty, it is already pretty
					return false, err
				}

				defer func() {
					if transactionFnErr == nil {
						if err := repo.DeleteRemote(properties.EphemeralRemote.Name); err != nil {
							h.logger.Error("deleting ephemeral remote", "error", err.Error())
							commit = false
							transactionFnErr = err
						}
					}
				}()
			}

			// pull remote content into local repo
			if err := repo.Pull(ctx, remoteName, opts...); err != nil {
				return false, werror.Wrap(gcode.ErrInternal, fmt.Errorf("pulling ostree repository: %w", err))
			}

			if err := repo.RegenerateSummary(); err != nil {
				return false, werror.Wrap(gcode.ErrInternal, fmt.Errorf("regenerating summary for ostree repository %s: %w", h.repoDir, err))
			}

			// List the refs in the repository and store in the repoSync
			loRefs, err := repo.ListRefsExt(libostree.ListRefsExtFlagsNone)
			if err != nil {
				return false, werror.Wrap(gcode.ErrInternal, fmt.Errorf("listing refs from ostree repository: %w", err))
			}
			repoSync := *h.repoSync.Load()
			repoSync.SyncedRefs = loRefs
			h.setRepoSync(&repoSync)

			return true, nil
		})
	}()

	return nil
}

func (h *Handler) GetRepositorySyncStatus(_ context.Context) (syncStatus *apiv1.SyncStatus, err error) {
	repoSync := h.repoSync.Load()
	if repoSync == nil {
		return nil, werror.Wrap(gcode.ErrNotFound, fmt.Errorf("repository sync status not available"))
	}

	var refs []apiv1.OSTreeRef
	for _, loRef := range repoSync.SyncedRefs {
		refs = append(refs, apiv1.OSTreeRef{
			Name:   loRef.Name,
			Commit: loRef.Checksum,
		})
	}

	return &apiv1.SyncStatus{
		Syncing:    repoSync.Syncing,
		StartTime:  utils.TimeToString(repoSync.StartTime),
		EndTime:    utils.TimeToString(repoSync.EndTime),
		SyncError:  repoSync.SyncError,
		SyncedRefs: refs,
	}, nil
}
