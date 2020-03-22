# tpccgen

A command line utility to generate/import/test via [go-tpc](https://github.com/pingcap/go-tpc)

## Install

```bash
go build
```

## Usage
```
Usage:
  tpccgen [flags]
  tpccgen [command]

Available Commands:
  all         Generate TPC-C CSV files and restore files by start import & lightning
  csv         Generate TPC-C CSV files
  restore     Restore files by start importer & lightning
  test        Run TPC-C test
  help        Help about any command

Flags:
  -h, --help   help for tpccgen

Use "tpccgen [command] --help" for more information about a command.
```

## Example

After deploying TiDB Lightning use the flowing command to generate CSV and restore files:

```
./tpccgen all --lightning-ip 172.16.5.70 --importer-ip 172.16.5.69 --deploy-dir /data1/deploy --db tpcc4  --warehouse 100 --tidb-ip "172.16.5.70"
```

If deploy multi TiDB Lightning:
```shelll
./tpccgen all --lightning-ip 172.16.5.70,172.16.5.74,172.16.5.75 --importer-ip 172.16.5.69,172.16.5.74,172.16.5.75 --deploy-dir /data1/deploy --db tpcc4  --warehouse 100 --tidb-ip "172.16.5.70"
```

Generate CSV files only:
```
./tpccgen csv --lightning-ip 172.16.5.70  --deploy-dir /data1/deploy --db tpcc4  --warehouse 100 --tidb-ip "172.16.5.70"
```

If the CSV has been generated, use the flowing command to start `lightning and `importer` to restart:
```
./tpccgen restore --lightning-ip 172.16.5.70 --importer-ip 172.16.5.69 --deploy-dir /data1/deploy
```
