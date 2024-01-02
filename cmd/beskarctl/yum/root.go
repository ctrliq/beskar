// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package yum

import (
	"github.com/spf13/cobra"
	"go.ciq.dev/beskar/cmd/beskarctl/ctl"
)

const (
	MissingRequiredFlagRepo     ctl.Err = "a repo must be specified"
	MissingRequiredFlagRegistry ctl.Err = "a registry must be specified"
)

var (
	repo     string
	registry string
	rootCmd  = &cobra.Command{
		Use: "yum",
		Aliases: []string{
			"y",
			"rpm",
			"dnf",
		},
		Short: "Operations related to yum repositories.",
	}
)

func RootCmd() *cobra.Command {
	rootCmd.AddCommand(
		PushCmd(),
		PushMetadataCmd(),
	)

	return rootCmd
}
