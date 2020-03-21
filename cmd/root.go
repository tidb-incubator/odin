package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd *cobra.Command

func init() {
	cobra.EnableCommandSorting = false

	rootCmd = &cobra.Command{
		Use:   "tpccgen",
		Short: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	rootCmd.AddCommand(
		newAllCmd(),
		newCSVCmd(),
		newRestoreCmd(),
		newTestCmd(),
	)
}

// Execute executes the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
