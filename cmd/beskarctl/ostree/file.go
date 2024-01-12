// SPDX-FileCopyrightText: Copyright (c) 2023-2024, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ostree

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use: "ostree",
	Aliases: []string{
		"o",
	},
	Short: "Operations related to ostree repositories.",
}

func RootCmd() *cobra.Command {
	rootCmd.AddCommand(
		PushCmd(),
	)

	return rootCmd
}
