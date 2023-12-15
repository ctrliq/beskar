package ostree

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasostree"
	"golang.org/x/sync/errgroup"
)

var (
	pushCmd = &cobra.Command{
		Use:   "push [directory]",
		Short: "Push an ostree repository.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if dir == "" {
				return ctl.Err("a directory must be specified")
			}

			if err := pushOSTreeRepository(dir, ctl.Repo(), ctl.Registry()); err != nil {
				return ctl.Errf("while pushing ostree repository: %s", err)
			}
			return nil
		},
	}
	jobCount int
)

func PushCmd() *cobra.Command {
	pushCmd.PersistentFlags().IntVarP(&jobCount, "jobs", "j", 10, "The repository to operate on.")
	return pushCmd
}

// pushOSTreeRepository walks a local ostree repository and pushes each file to the given registry.
// dir is the root directory of the ostree repository, i.e., the directory containing the summary file.
// repo is the name of the ostree repository.
// registry is the registry to push to.
func pushOSTreeRepository(dir, repo, registry string) error {
	// Prove that we were given the root directory of an ostree repository
	// by checking for the existence of the summary file.
	fileInfo, err := os.Stat(filepath.Join(dir, orasostree.KnownFileSummary))
	if err != nil || fileInfo.IsDir() {
		return fmt.Errorf("%s file not found in %s", orasostree.KnownFileSummary, dir)
	}

	// Create a worker pool to push each file in the repository concurrently.
	// ctx will be cancelled on error, and the error will be returned.
	eg, ctx := errgroup.WithContext(context.Background())
	eg.SetLimit(jobCount)

	// Walk the directory tree, skipping directories and pushing each file.
	if err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		// If there was an error with the file, return it.
		if err != nil {
			return fmt.Errorf("while walking %s: %w", path, err)
		}

		// Skip directories.
		if d.IsDir() {
			return nil
		}

		if ctx.Err() != nil {
			// Skip remaining files because our context has been cancelled.
			// We could return the error here, but we want to exclusively handle that error in our call to eg.Wait().
			// This is because we would never be able to handle an error returned from the last job.
			return filepath.SkipAll
		}

		eg.Go(func() error {
			if err := push(dir, path, repo, registry); err != nil {
				return fmt.Errorf("while pushing %s: %w", path, err)
			}
			return nil
		})

		return nil
	}); err != nil {
		// We should only receive here if filepath.WalkDir() returns an error.
		// Push errors are handled below.
		return fmt.Errorf("while walking %s: %w", dir, err)
	}

	// Wait for all workers to finish.
	// If any worker returns an error, eg.Wait() will return that error.
	return eg.Wait()
}

func push(repoRootDir, path, repo, registry string) error {
	pusher, err := orasostree.NewOSTreePusher(repoRootDir, path, repo, name.WithDefaultRegistry(registry))
	if err != nil {
		return fmt.Errorf("while creating StaticFile pusher: %w", err)
	}

	path = strings.TrimPrefix(path, repoRootDir)
	path = strings.TrimPrefix(path, "/")
	fmt.Printf("Pushing %s to %s\n", path, pusher.Reference())

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
