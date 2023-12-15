package yum

import (
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/pkg/oras"
	"go.ciq.dev/beskar/pkg/orasrpm"
)

var pushCmd = &cobra.Command{
	Use:   "push [rpm filepath]",
	Short: "Push a yum repository to a registry.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rpm := args[0]
		if rpm == "" {
			return ctl.Err("an RPM package must be specified")
		}

		if err := push(rpm, ctl.Repo(), ctl.Registry()); err != nil {
			return ctl.Errf("while pushing RPM package: %s", err)
		}
		return nil
	},
}

func PushCmd() *cobra.Command {
	return pushCmd
}

func push(rpmPath, repo, registry string) error {
	pusher, err := orasrpm.NewRPMPusher(rpmPath, repo, name.WithDefaultRegistry(registry))
	if err != nil {
		return fmt.Errorf("while creating RPM pusher: %w", err)
	}

	fmt.Printf("Pushing %s to %s\n", rpmPath, pusher.Reference())

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
