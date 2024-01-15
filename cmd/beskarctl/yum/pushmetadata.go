// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

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

// yum push-metadata
var (
	pushMetadataCmd = &cobra.Command{
		Use:   "push-metadata [metadata filepath]",
		Short: "Push yum repository metadata to a registry.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			metadata := args[0]
			if metadata == "" {
				return ctl.Err("a metadata file must be specified")
			} else if registry == "" {
				return MissingRequiredFlagRegistry
			} else if repo == "" {
				return MissingRequiredFlagRepo
			} else if pushMetadataType == "" {
				return ctl.Err("a metadata type must be specified")
			}

			if err := pushMetadata(metadata, pushMetadataType, ctl.Repo(), ctl.Registry()); err != nil {
				return ctl.Errf("while pushing metadata: %s", err)
			}
			return nil
		},
	}
	pushMetadataType string
)

func PushMetadataCmd() *cobra.Command {
	pushMetadataCmd.Flags().StringVarP(&pushMetadataType, "type", "t", "", "type")
	return pushMetadataCmd
}

func pushMetadata(metadataPath, dataType, repo, registry string) error {
	pusher, err := orasrpm.NewRPMExtraMetadataPusher(metadataPath, repo, dataType, name.WithDefaultRegistry(registry))
	if err != nil {
		return fmt.Errorf("while creating RPM metadata pusher: %w", err)
	}

	fmt.Printf("Pushing %s to %s\n", metadataPath, pusher.Reference())

	return oras.Push(pusher, remote.WithAuthFromKeychain(authn.DefaultKeychain))
}
