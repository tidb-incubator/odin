# tpccgen

A command line utility to generate/import/test via [go-tpc](https://github.com/pingcap/go-tpc)

## Install

```bash
go build -o tpccgen main.go
```

## Flags

```bash
Usage of ./tpccgen:
  -all
        do all the actions (default true)
  -csv
        generate tpcc csv files
  -data-dir string
        data source directory of lightning
  -db string
        test database name (default "tpcc")
  -deploy-dir string
        directory path of cluster deployment
  -download-url string
        url of the go-tpc binary to download (default "https://github.com/pingcap/go-tpc/releases/download/v1.0.0/go-tpc_1.0.0_linux_amd64.tar.gz")
  -importer-ip string
        ip address of tikv-importer
  -lightning-ip string
        ip address of tidb-lightnings
  -restore
        start lightning, importer and restore files
  -skip-download
        skip downloading the go-tpc binary
  -test
        run tpcc test
  -threads int
        number of threads (default 40)
  -tidb-ip string
        ip of tidb-server (default "127.0.0.1")
  -tidb-port string
        port of tidb-server (default "4000")
  -warehouse int
        number of warehouses (default 100)
```

## Usage

By default, tpccgen will automatically take 3 actions:

1. download [go-tpc](https://github.com/pingcap/go-tpc) binary
2. generate csv data
3. import csv data through [tidb-lightning](https://github.com/pingcap/tidb-lightning)

You can also add some flags you want. For example:

```bash
./tpccgen --lightning-ip 172.16.5.90 --importer-ip 172.16.5.89 --data-dir /data/lightning --tidb-ip 172.16.5.84 --tidb-port 4111 --deploy-dir /data/deploy --db test --threads 40 --warehouse 10
```

If you want to perform tpcc test, you must add `--test` flag just like the command below:

```bash
./tpccgen --test --lightning-ip 172.16.5.90 --tidb-ip 172.16.5.84 --tidb-port 4111 --db test --threads 40 --warehouse 10
```
