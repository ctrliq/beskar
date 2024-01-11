package ostreerepository

import (
	"context"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/internal/plugins/ostree/pkg/libostree"
	"go.ciq.dev/beskar/pkg/orasostree"
	"os"
	"path/filepath"
)

// checkRepoExists checks if the ostree repository exists in beskar.
func (h *Handler) checkRepoExists(_ context.Context) bool {
	// Check if repo already exists
	configTag := orasostree.MakeTag(orasostree.FileConfig)
	configRef := filepath.Join(h.Repository, "file:"+configTag)
	_, err := h.GetManifestDigest(configRef)
	return err == nil
}

func (h *Handler) BeginLocalRepoTransaction(ctx context.Context, transactor func(ctx context.Context, repo *ostree.Repo) error) error {
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

	// We will always use archive mode here
	repo, err := ostree.Init(h.repoDir, ostree.RepoModeArchive)
	if err != nil {
		return ctl.Errf("while opening ostree repository %s: %s", h.repoDir, err)
	}

	// Ad beskar as a remote so that we can pull from it
	beskarServiceURL := h.Params.GetBeskarServiceHostPort()
	if err := repo.AddRemote(beskarRemoteName, beskarServiceURL, ostree.NoGPGVerify()); err != nil {
		return ctl.Errf("while adding remote to ostree repository %s: %s", beskarRemoteName, err)
	}

	// pull remote content into local repo from beskar
	if h.checkRepoExists(ctx) {
		if err := repo.Pull(ctx, beskarRemoteName, ostree.NoGPGVerify()); err != nil {
			return ctl.Errf("while pulling ostree repository from %s: %s", beskarRemoteName, err)
		}
	}

	// Execute the transaction
	if err := transactor(ctx, repo); err != nil {
		return ctl.Errf("while executing transaction: %s", err)
	}

	if err := repo.RegenerateSummary(); err != nil {
		return ctl.Errf("while regenerating summary for ostree repository %s: %s", h.repoDir, err)
	}

	// Remove the internal beskar remote so that external clients can't pull from it, not that it would work.
	if err := repo.DeleteRemote(beskarRemoteName); err != nil {
		return ctl.Errf("while deleting remote %s: %s", beskarRemoteName, err)
	}

	// Close the local
	// Push local repo to beskar using OSTreePusher
	if err := orasostree.PushOSTreeRepository(
		ctx,
		h.repoDir,
		h.Repository,
		100,
		h.Params.NameOptions...,
	); err != nil {
		return err
	}

	// Clean up the disk
	if err := os.RemoveAll(h.repoDir); err != nil {
		return ctl.Errf("while removing local repo %s: %s", h.repoDir, err)
	}

	return nil
}
