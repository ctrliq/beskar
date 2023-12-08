package static

import (
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use: "static",
		Aliases: []string{
			"file",
			"s",
		},
		Short: "Operations related to static files.",
	}
)

func RootCmd() *cobra.Command {
	rootCmd.AddCommand(
		PushCmd(),
	)

	return rootCmd
}
