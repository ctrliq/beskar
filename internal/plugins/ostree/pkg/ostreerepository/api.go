package ostreerepository

import (
	"context"
	"errors"
	"fmt"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/internal/plugins/ostree/pkg/libostree"
	"go.ciq.dev/beskar/pkg/orasostree"
	apiv1 "go.ciq.dev/beskar/pkg/plugins/ostree/api/v1"
	"go.ciq.dev/beskar/pkg/utils"
	"golang.org/x/sync/errgroup"
	"os"
	"path/filepath"
)

var (
	errHandlerNotStarted   = errors.New("handler not started")
	errProvisionInProgress = errors.New("provision in progress")
	errSyncInProgress      = errors.New("sync in progress")
	errDeleteInProgress    = errors.New("delete in progress")
)

func (h *Handler) CreateRepository(ctx context.Context, properties *apiv1.OSTreeRepositoryProperties) (err error) {
	h.logger.Debug("creating repository", "repository", h.Repository)
	// Validate request
	if len(properties.Remotes) == 0 {
		return ctl.Errf("at least one remote is required")
	}

	// Transition to provisioning state
	if err := h.setState(StateProvisioning); err != nil {
		return err
	}
	defer h.clearState()

	// Check if repo already exists
	if h.checkRepoExists(ctx) {
		return ctl.Errf("repository %s already exists", h.Repository)
	}

	if err := h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *ostree.Repo) error {

		// Add user provided remotes
		// We do not need to add beskar remote here
		for _, remote := range properties.Remotes {
			var opts []ostree.Option
			if remote.NoGPGVerify {
				opts = append(opts, ostree.NoGPGVerify())
			}
			if err := repo.AddRemote(remote.Name, remote.RemoteURL, opts...); err != nil {
				return ctl.Errf("while adding remote to ostree repository %s: %s", remote.Name, err)
			}
		}
		return nil

	}); err != nil {
		return err
	}

	return nil
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
	if h.checkRepoExists(ctx) {
		return ctl.Errf("repository %s already exists", h.Repository)
	}

	go func() {
		defer func() {
			h.clearState()
			if err == nil {
				// stop the repo handler and trigger cleanup
				h.Stop()
			}
		}()
		h.logger.Debug("deleting repository", "repository", h.Repository)

		if err := h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *ostree.Repo) error {

			// Create a worker pool to deleting each file in the repository concurrently.
			// ctx will be cancelled on error, and the error will be returned.
			eg, ctx := errgroup.WithContext(ctx)
			eg.SetLimit(100)

			// Walk the directory tree, skipping directories and deleting each file.
			if err := filepath.WalkDir(h.repoDir, func(path string, d os.DirEntry, err error) error {
				// If there was an error with the file, return it.
				if err != nil {
					return fmt.Errorf("while walking %s: %w", path, err)
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
					digest := orasostree.MakeTag(filename)
					digestRef := filepath.Join(h.Repository, "file:"+digest)
					if err := h.DeleteManifest(digestRef); err != nil {
						h.logger.Error("deleting file from beskar", "repository", h.Repository, "error", err.Error())
					}

					return nil
				})

				return nil
			}); err != nil {
				return nil
			}

			return eg.Wait()

		}); err != nil {
			h.logger.Error("while deleting repository", "repository", h.Repository, "error", err.Error())
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

	if h.checkRepoExists(ctx) {
		return ctl.Errf("repository %s does not exist", h.Repository)
	}

	if err := h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *ostree.Repo) error {
		// Add user provided remote
		var opts []ostree.Option
		if remote.NoGPGVerify {
			opts = append(opts, ostree.NoGPGVerify())
		}
		if err := repo.AddRemote(remote.Name, remote.RemoteURL, opts...); err != nil {
			return ctl.Errf("while adding remote to ostree repository %s: %s", remote.Name, err)
		}

		return nil

	}); err != nil {
		return err
	}

	return nil
}

func (h *Handler) SyncRepository(ctx context.Context, request *apiv1.OSTreeRepositorySyncRequest) (err error) {
	// Transition to syncing state
	if err := h.setState(StateSyncing); err != nil {
		return err
	}

	// Spin up pull worker
	go func() {
		defer func() {
			h.clearState()
			h.logger.Debug("repository sync complete", "repository", h.Repository, "request", request)
		}()

		if err := h.BeginLocalRepoTransaction(ctx, func(ctx context.Context, repo *ostree.Repo) error {
			// Pull the latest changes from the remote.
			opts := []ostree.Option{
				ostree.Depth(request.Depth),
			}
			if len(request.Refs) > 0 {
				opts = append(opts, ostree.Refs(request.Refs...))
			}

			// pull remote content into local repo
			if err := repo.Pull(ctx, request.Remote, opts...); err != nil {
				return ctl.Errf("while pulling ostree repository from %s: %s", request.Remote, err)
			}

			return nil

		}); err != nil {
			h.logger.Error("repository sync", "repository", h.Repository, "request", request, "error", err.Error())
		}
	}()

	return nil
}

func (h *Handler) GetRepositorySyncStatus(_ context.Context) (syncStatus *apiv1.SyncStatus, err error) {
	repoSync := h.getRepoSync()
	return &apiv1.SyncStatus{
		Syncing:   repoSync.Syncing,
		StartTime: utils.TimeToString(repoSync.StartTime),
		EndTime:   utils.TimeToString(repoSync.EndTime),
		SyncError: repoSync.SyncError,
	}, nil
}
