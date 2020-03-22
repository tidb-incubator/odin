package cmd

import (
	"github.com/XuHuaiyu/tpccgen/tpcc"
	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {

	var time string

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run TPC-C test",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			err = tpcc.FetchTpcc(lightningIPs, downloadURL, skipDownload)
			if err != nil {
				return err
			}

			err = tpcc.RunTPCCTest(lightningIPs[0], tidbIP, tidbPort, dbName, warehouse, threads, time)
			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&time, "time", "1h", "TPC run time")

	setFlag(cmd)

	return cmd
}
