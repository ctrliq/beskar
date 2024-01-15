// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ctl

import (
	"os"

	"github.com/spf13/cobra"
)

const (
	ErrMissingFlagRepo     = Err("missing repo flag")
	ErrMissingFlagRegistry = Err("missing registry flag")
)

const (
	FlagNameRepo     = "repo"
	FlagNameRegistry = "registry"
)

// RegisterFlags registers the flags that are common to all commands.
func RegisterFlags(cmd *cobra.Command) {
	// Flags that are common to all commands.
	cmd.PersistentFlags().String(FlagNameRepo, "", "The repository to operate on.")
	cmd.PersistentFlags().String(FlagNameRegistry, "", "The registry to operate on.")
}

// Repo returns the repository name from the command line.
// If the repository is not specified, the command will exit with an error.
func Repo() string {
	repo, err := rootCmd.Flags().GetString(FlagNameRepo)
	if err != nil || repo == "" {
		rootCmd.PrintErrln(ErrMissingFlagRepo)
		os.Exit(1)
	}

	return repo
}

// Registry returns the registry name from the command line.
// If the registry is not specified, the command will exit with an error.
func Registry() string {
	registry, err := rootCmd.Flags().GetString(FlagNameRegistry)
	if err != nil || registry == "" {
		rootCmd.PrintErrln(ErrMissingFlagRegistry)
		os.Exit(1)
	}

	return registry
}
