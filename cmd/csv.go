package cmd

import (
	"github.com/XuHuaiyu/tpccgen/tpcc"
	"github.com/spf13/cobra"
)

var (
	lightningIPs []string
	importerIPs  []string
	dataDirs     []string
	deployDir    string
	tidbIP       string
	tidbPort     int
	dbName       string
	skipDownload bool
	downloadURL  string
	warehouse    int
	threads      int
)

func setFlag(cmd *cobra.Command) {
	cmd.Flags().StringSliceVar(&lightningIPs, "lightning-ip", nil, "ip address of tidb-lightning")
	cmd.Flags().StringSliceVar(&importerIPs, "importerIPs", nil, "ip address of tikv-importer")
	cmd.Flags().StringSliceVar(&dataDirs, "data-dir", nil, "data source directory of lightning")
	cmd.Flags().StringVar(&deployDir, "deploy-dir", "", "directory path of cluster deployment")
	cmd.Flags().StringVar(&dbName, "db", "tpcc", "test database name")
	cmd.Flags().StringVar(&tidbIP, "tidb-ip", "127.0.0.1", "ip of TiDB server")
	cmd.Flags().IntVar(&tidbPort, "tidb-port", 4000, "port of TiDB server")
	cmd.Flags().BoolVar(&skipDownload, "skip-download", false, "skip downloading the go-tpc binary")
	cmd.Flags().StringVar(&downloadURL, "download-url", "https://github.com/pingcap/go-tpc/releases/download/v1.0.3/go-tpc_1.0.3_linux_amd64.tar.gz", "url of the go-tpc binary to download")
	cmd.Flags().IntVar(&warehouse, "warehouse", 100, "number of warehouses")
	cmd.Flags().IntVar(&threads, "threads", 40, "number of threads of go-tpc")

	_ = cmd.MarkFlagRequired("lightning-ip")
}

func newCSVCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "csv",
		Short: "generate TPC-C CSV files",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			dataDirs, err = tpcc.SetDataDirs(deployDir, lightningIPs, dataDirs)
			if err != nil {
				return err
			}

			err = tpcc.DropDB(tidbIP, tidbPort, dbName)
			if err != nil {
				return err
			}

			err = tpcc.FetchTpcc(dataDirs, lightningIPs, downloadURL, skipDownload)
			if err != nil {
				return err
			}

			err = tpcc.GenCSV(lightningIPs, dataDirs, tidbIP, tidbPort, dbName, warehouse, threads)
			if err != nil {
				return err
			}

			return nil
		},
	}

	setFlag(cmd)

	return cmd
}
