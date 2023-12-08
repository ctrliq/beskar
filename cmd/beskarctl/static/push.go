package static

import (
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasfile"
)

var (
	pushCmd = &cobra.Command{
		Use:   "push [file]",
		Short: "Push a file.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			file := args[0]
			if file == "" {
				return ctl.Err("file must be specified")
			}

			if err := push(file, ctl.Repo(), ctl.Registry()); err != nil {
				return ctl.Errf("while pushing static file: %s", err)
			}
			return nil
		},
	}
)

func PushCmd() *cobra.Command {
	return pushCmd
}

func push(filepath, repo, registry string) error {
	pusher, err := orasfile.NewStaticFilePusher(filepath, repo, name.WithDefaultRegistry(registry))
	if err != nil {
		return fmt.Errorf("while creating StaticFile pusher: %w", err)
	}

	fmt.Printf("Pushing %s to %s\n", filepath, pusher.Reference())

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
