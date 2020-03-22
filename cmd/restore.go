package cmd

import (
	"github.com/XuHuaiyu/tpccgen/tpcc"
	"github.com/spf13/cobra"
)

func newRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "start lightning, importer and restore files",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := tpcc.RestoreData(importerIPs, lightningIPs, deployDir)
			if err != nil {
				return err
			}

			return nil
		},
	}

	setFlag(cmd)

	_ = cmd.MarkFlagRequired("deploy-dir")

	return cmd
}
