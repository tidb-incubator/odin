package cmd

import (
	"github.com/XuHuaiyu/tpccgen/tpcc"
	"github.com/spf13/cobra"
)

func newRestoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore files by start importer & lightning",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := tpcc.RestoreData(importerIPs, lightningIPs, deployDir)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringSliceVar(&lightningIPs, "lightning-ip", nil, "ip address of tidb-lightning")
	cmd.Flags().StringSliceVar(&importerIPs, "importer-ip", nil, "ip address of tikv-importer")
	cmd.Flags().StringVar(&deployDir, "deploy-dir", "", "directory path of cluster deployment")

	_ = cmd.MarkFlagRequired("deploy-dir")
	_ = cmd.MarkFlagRequired("lightning-ip")
	_ = cmd.MarkFlagRequired("importer-ip")

	return cmd
}
