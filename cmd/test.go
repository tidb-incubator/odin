package cmd

import (
	"github.com/XuHuaiyu/tpccgen/tpcc"
	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "run TPC-C test",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			err = tpcc.DropDB(tidbIP, tidbPort, dbName)
			if err != nil {
				return err
			}

			err = tpcc.FetchTpcc(dataDirs, lightningIPs, downloadURL, skipDownload)
			if err != nil {
				return err
			}

			err = tpcc.RunTPCCTest(lightningIPs[0], tidbIP, tidbPort, dbName, warehouse, threads)
			if err != nil {
				return err
			}

			return nil
		},
	}

	setFlag(cmd)

	return cmd
}
