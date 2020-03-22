
## 一键 TPC-C 造数导入工具说明

## 背景

本工具旨在降低[对 TiDB 进行 TPC-C 测试](https://pingcap.com/docs-cn/stable/benchmark/how-to-run-tpcc/#%E4%BF%AE%E6%94%B9-benchmarksql-%E7%9A%84%E9%85%8D%E7%BD%AE%E6%96%87%E4%BB%B6)时的人为操作成本，将 TPC-C 生成数据及导入的中间流程进行封装，提供 TPC-C 测试时的一键造数导入功能。

## 安装

```bash
go build
```

## 原理

该工具将如下步骤进行封装

1. 下载 [go-tpc](https://github.com/pingcap/go-tpc) 到 `lightning` 所在机器的临时目录。

2. 在 lightning 所在机器使用 `go-tpc tpcc prepare`  命令生成 TPC-C 数据的 CSV 文件。

3. 在 `importer` 目录下执行 `scripts/start_importer.sh`，启动 `importer`。

4. 修改 `tidb-lighting.toml` 与数据源相关的配置。

5. 在 `lightning` 目录下执行 `scripts/start_lightning.sh`，开始导入数据。


若部署了多台 `lightning`，会在多台 `lightning` 所在服务器上并行生成 CSV，每台服务器分别生成和导入不同 table 对应的 CSV 文件。 

目前 `lightning` 单表导入的瓶颈在 `order_line` 表，其它表可以分别使用另外两个 `lightning` 导入使得比导入 `order_line` 表花费的时间少，所以最多只需要 3 个 `lightning` 并行导入来提高整体导入的速度。

当部署了两台 `lightning` 会分别生成如下表的对应 CSV 文件，目前 `order_line` 与 `order` 表必须在同一台机器生成。

- lightning 1 生成:
  - orders, order_line

- lightning 2 生成:
  - stock, customer, district, history, item, new_order, warehouse

当部署了三台 lightning 会分别生成如下表的对应 CSV 文件:

- lightning 1 生成:
  - orders, order_line

- lightning 2 生成:
  - stock

- lightning 3 生成:
  - customer, district, history, item, new_order, warehouse

`lightning` 内部自动对单个大的 CSV 文件进行切分，解决了导入前需要手动切分大 CSV 文件的问题。

启动 `tidb-lightning` 前会先自动修改 `tide-lightning.toml` 如下与数据源相关配置, 避免了需要手动根据 CSV 格式修改对应配置：

```toml
[mydumper]
no-schema = true # go-tpc 生成数据会同时建好 schema
strict-format = true

[mydumper.csv]
backslash-escape = false
delimiter = ""
header = false
not-null = false
null = "NULL"
separator = ","
trim-last-separator = false
```

## 使用说明
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

## 注意事项

* 该工具假设 tidb-lightning 和 tikv-importer 的 binary 已部署完成，请参考[TiDB Lightning 部署与执行](https://pingcap.com/docs-cn/stable/reference/tools/tidb-lightning/deployment/)。
* 当存在多 lightning 实例时，需确保这些 lightning 实例位于不同的物理机上。
* 该工具仅实现了数据生成和导入流程的启动，导入是否完成需要手动从 lightning 日志或监控中获取。
* 该工具需要可以 ssh 到 `lightning` 与 `importer` 机器，推荐在中控机运行。

## 示例

部署好 TiDB Lightning 后使用如下命令自动生成数据与导入：

```
./tpccgen all --lightning-ip 172.16.5.70 --importer-ip 172.16.5.69 --deploy-dir /data1/deploy --db tpcc4  --warehouse 100 --tidb-ip "172.16.5.70"
```

如果部署了多套 TiDB Lightning:
```shelll
./tpccgen all --lightning-ip 172.16.5.70,172.16.5.74,172.16.5.75 --importer-ip 172.16.5.69,172.16.5.74,172.16.5.75 --deploy-dir /data1/deploy --db tpcc4  --warehouse 100 --tidb-ip "172.16.5.70"
```

只生成 CSV 数据：
```
./tpccgen csv --lightning-ip 172.16.5.70  --deploy-dir /data1/deploy --db tpcc4  --warehouse 100 --tidb-ip "172.16.5.70"
```

假设已生成数据，如下命令启动 `lightning` 与 `importer` 开始导入：
```
./tpccgen restore --lightning-ip 172.16.5.70 --importer-ip 172.16.5.69 --deploy-dir /data1/deploy
```
