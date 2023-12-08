package ostree

import (
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasostree"
	"os"
	"path/filepath"
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
)

func PushCmd() *cobra.Command {
	return pushCmd
}

func pushOSTreeRepository(dir, repo, registry string) error {
	// Prove that we were given the root directory of an ostree repository
	// by checking for the existence of the summary file.
	fileInfo, err := os.Stat(filepath.Join(dir, orasostree.KnownFileSummary))
	if err != nil || fileInfo.IsDir() {
		return fmt.Errorf("%s file not found in %s", orasostree.KnownFileSummary, dir)
	}

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("while walking %s: %w", path, err)
		}

		if info.IsDir() {
			return nil
		}

		if err := push(path, repo, registry); err != nil {
			return fmt.Errorf("while pushing %s: %w", path, err)
		}

		return nil
	})
}

func push(filepath, repo, registry string) error {
	pusher, err := orasostree.NewOSTreePusher(filepath, repo, name.WithDefaultRegistry(registry))
	if err != nil {
		return fmt.Errorf("while creating StaticFile pusher: %w", err)
	}

	fmt.Printf("Pushing %s to %s\n", filepath, pusher.Reference())

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
