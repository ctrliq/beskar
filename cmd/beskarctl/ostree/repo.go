// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostree

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/spf13/cobra"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
	"go.ciq.dev/beskar/pkg/orasostree"
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

			repoPusher := orasostree.NewOSTreeRepositoryPusher(context.Background(), dir, ctl.Repo(), jobCount)
			repoPusher = repoPusher.WithNameOptions(name.WithDefaultRegistry(ctl.Registry()))
			repoPusher = repoPusher.WithRemoteOptions(remote.WithAuthFromKeychain(authn.DefaultKeychain))
			if err := repoPusher.Push(); err != nil {
				return ctl.Errf("while pushing ostree repository: %s", err)
			}
			return nil
		},
	}
	jobCount int
)

func PushCmd() *cobra.Command {
	pushCmd.Flags().IntVarP(
		&jobCount,
		"jobs",
		"j",
		10,
		"The number of concurrent jobs to use for pushing the repository.",
	)
	return pushCmd
}
