package ostree

import (
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use: "ostree",
		Aliases: []string{
			"o",
		},
		Short: "Operations related to ostree repositories.",
	}
)

func RootCmd() *cobra.Command {
	rootCmd.AddCommand(
		PushCmd(),
	)

	return rootCmd
}
