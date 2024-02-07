// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostreerepository

import (
	"context"
	"os"
	"path"
	"path/filepath"

	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/internal/plugins/ostree/pkg/libostree"
	"go.ciq.dev/beskar/pkg/orasostree"
)

// checkRepoExists checks if the ostree repository exists in beskar.
func (h *Handler) checkRepoExists(_ context.Context) bool {
	// Check if repo already exists
	configTag := orasostree.MakeTag(orasostree.FileConfig)
	configRef := filepath.Join(h.Repository, "file:"+configTag)
	_, err := h.GetManifestDigest(configRef)
	return err == nil
}

type (
	TransactionFn      func(ctx context.Context, repo *libostree.Repo) (commit bool, err error)
	TransactionOptions struct {
		skipPull bool
	}
)

type TransactionOption func(*TransactionOptions)

func SkipPull() TransactionOption {
	return func(opts *TransactionOptions) {
		opts.skipPull = true
	}
}

// BeginLocalRepoTransaction executes a transaction against the local ostree repository.
// The transaction is executed in a temporary directory in which the following steps are performed:
// 1. The local ostree repository is opened.
// 2. The beskar remote is added to the local ostree repository.
// 3. The beskar version of the repo is pulled into the local ostree repository.
// 4. The transactorFn is executed.
// 5. If the transactorFn returns true, the local ostree repository is pushed to beskar. If false, all local changes are discarded.
// 6. The temporary directory is removed.
func (h *Handler) BeginLocalRepoTransaction(ctx context.Context, tFn TransactionFn, opts ...TransactionOption) error {
	options := TransactionOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	// We control the local repo lifecycle here, so we need to lock it.
	h.repoLock.Lock()
	defer h.repoLock.Unlock()

	// Open the local repo
	// Create the repository directory
	if err := os.MkdirAll(h.repoDir, 0o700); err != nil {
		// If the directory already exists, we can continue
		if !os.IsExist(err) {
			return ctl.Errf("create repository dir: %s", err)
		}
	}

	// Clean up the disk when we are done
	defer func() {
		if err := os.RemoveAll(h.repoDir); err != nil {
			h.logger.Error("removing local repo", "repo", h.repoDir, "error", err)
		}
	}()

	// We will always use archive mode here
	// Note that we are not using the returned repo pointer here.  We will re-open the repo later.
	_, err := libostree.Init(h.repoDir, libostree.RepoModeArchive)
	if err != nil {
		return ctl.Errf("initializing ostree repository %s: %s", h.repoDir, err)
	}

	// It is necessary to pull the config from beskar before we can add the beskar remote.  This is because config files
	// are unique the instance of a repo you are interacting with. Meaning, remotes are not pulled with the repo's data.
	if err := h.pullFile(ctx, orasostree.FileConfig); err != nil {
		h.logger.Debug("no config found in beskar", "error", err)
	}

	// Re-open the local repo
	// We need to re-open the repo here because we just pulled the config from beskar. If we don't re-open the repo, the
	// config we just manually pulled down will not be loaded into memory.
	repo, err := libostree.Open(h.repoDir)
	if err != nil {
		return ctl.Errf("opening ostree repository %s: %s", h.repoDir, err)
	}

	// Add beskar as a remote so that we can pull from it
	beskarServiceURL := "http://" + h.Params.GetBeskarRegistryHostPort() + path.Join("/", h.Repository, "repo")
	if err := repo.AddRemote(beskarRemoteName, beskarServiceURL, libostree.NoGPGVerify()); err != nil {
		return ctl.Errf("adding remote to ostree repository %s: %s", beskarRemoteName, err)
	}

	// pull remote content into local repo from beskar
	if !options.skipPull && h.checkRepoExists(ctx) {
		if err := repo.Pull(
			ctx,
			beskarRemoteName,
			libostree.NoGPGVerify(),
			libostree.Flags(libostree.Mirror|libostree.TrustedHTTP),
		); err != nil {
			return ctl.Errf("pulling ostree repository from %s: %s", beskarRemoteName, err)
		}
	}

	// Execute the transaction
	commit, err := tFn(ctx, repo)
	if err != nil {
		return ctl.Errf("executing transaction: %s", err)
	}

	// Commit the changes to beskar if the transaction deems it necessary
	if commit {
		// Remove the internal beskar remote so that external clients can't pull from it, not that it would work.
		if err := repo.DeleteRemote(beskarRemoteName); err != nil {
			return ctl.Errf("deleting remote %s: %s", beskarRemoteName, err)
		}

		// Close the local
		// Push local repo to beskar using OSTreePusher
		repoPusher := orasostree.NewOSTreeRepositoryPusher(
			ctx,
			h.repoDir,
			h.Repository,
			h.Params.Sync.MaxWorkerCount,
		)
		repoPusher = repoPusher.WithLogger(h.logger)
		repoPusher = repoPusher.WithNameOptions(h.Params.NameOptions...)
		repoPusher = repoPusher.WithRemoteOptions(h.Params.RemoteOptions...)
		if err := repoPusher.Push(); err != nil {
			return ctl.Errf("pushing ostree repository: %s", err)
		}
	}

	return nil
}
