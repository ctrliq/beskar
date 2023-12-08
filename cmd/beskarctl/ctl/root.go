package ctl

import (
	"fmt"
	"github.com/spf13/cobra"
	"go.ciq.dev/beskar/cmd/beskarctl/ostree"
	"go.ciq.dev/beskar/cmd/beskarctl/static"
	"go.ciq.dev/beskar/cmd/beskarctl/yum"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "beskarctl",
	Short: "Operations related to beskar.",
}

func Execute() {
	RegisterFlags(rootCmd)

	rootCmd.AddCommand(
		yum.RootCmd(),
		static.RootCmd(),
		ostree.RootCmd(),
	)

	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
