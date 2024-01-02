// SPDX-FileCopyrightText: Copyright (c) 2023, CIQ, Inc. All rights reserved
// SPDX-License-Identifier: Apache-2.0

package ctl

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "beskarctl",
	Short: "Operations related to beskar.",
}

func Execute(cmds ...*cobra.Command) {
	RegisterFlags(rootCmd)

	rootCmd.AddCommand(
		cmds...,
	)

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
