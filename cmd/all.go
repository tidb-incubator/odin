package cmd

import (
	"github.com/XuHuaiyu/tpccgen/tpcc"
	"github.com/spf13/cobra"
)

func newAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "all",
		Short: "do all the actions, test is not included",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			dataDirs, err = tpcc.SetDataDirs(deployDir, lightningIPs, dataDirs)
			if err != nil {
				return err
			}

			err = tpcc.FetchTpcc(lightningIPs, downloadURL, skipDownload)
			if err != nil {
				return err
			}

			err = tpcc.DropDB(tidbIP, tidbPort, dbName)
			if err != nil {
				return err
			}

			err = tpcc.GenCSV(lightningIPs, dataDirs, tidbIP, tidbPort, dbName, warehouse, threads)
			if err != nil {
				return err
			}

			err = tpcc.RestoreData(importerIPs, lightningIPs, deployDir)
			if err != nil {
				return err
			}

			return nil
		},
	}

	setFlag(cmd)

	return cmd
}
