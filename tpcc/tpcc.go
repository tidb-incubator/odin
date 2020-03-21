package tpcc

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	// for mysql
	_ "github.com/go-sql-driver/mysql"
	"golang.org/x/sync/errgroup"
)

const (
	nmAll     = "all"
	nmCSV     = "csv"
	nmTest    = "test"
	nmRestore = "restore"

	nmWarehouse = "warehouse"
	nmThreads   = "threads"

	nmTiDBIP    = "tidb-ip"
	nmTiDBPort  = "tidb-port"
	nmDeployDir = "deploy-dir"

	//nmAnsibleDir    = "ansible-dir"
	nmLightningIP = "lightning-ip"
	nmDataDir     = "data-dir"

	nmImporterIP = "importer-ip"
	nmDB         = "db"
	downloadURL  = "download-url"
	skipDownload = "skip-download"

	nmtime = "time"
)

var (
	all     = flag.Bool(nmAll, true, "do all the actions, test is not included")
	csv     = flag.Bool(nmCSV, false, "generate tpcc csv files")
	test    = flag.Bool(nmTest, false, "run tpcc test")
	restore = flag.Bool(nmRestore, false, "start lightning, importer and restore files")

	tidbIP    = flag.String(nmTiDBIP, "127.0.0.1", "ip of tidb-server")
	tidbPort  = flag.String(nmTiDBPort, "4000", "port of tidb-server")
	deployDir = flag.String(nmDeployDir, "", "directory path of cluster deployment")

	warehouse = flag.Int64(nmWarehouse, 100, "number of warehouses")
	threads   = flag.Int64(nmThreads, 40, "number of threads of go-tpc")

	//ansibleDir = flag.String(nmAnsibleDir, "", "ansible directory path")

	// TODO: If there is only one lightning, we do not need this var, we can fetch it from ansible.
	lightningIP = flag.String(nmLightningIP, "", "ip address of tidb-lightnings")
	dataDir     = flag.String(nmDataDir, "", "data source directory of lightning")
	importerIP  = flag.String(nmImporterIP, "", "ip address of tikv-importer")

	dbName          = flag.String(nmDB, "tpcc", "test database name")
	goTPCFile       = flag.String(downloadURL, "https://github.com/pingcap/go-tpc/releases/download/v1.0.2/go-tpc_1.0.2_linux_amd64.tar.gz", "url of the go-tpc binary to download")
	skipDownloading = flag.Bool(skipDownload, false, "skip downloading the go-tpc binary")

	tpcruntime = flag.String(nmtime, "1h", "tpc run time")
)

// SetDataDirs Set dataDirs as deployDir/mydumper default if it's not setted.
func SetDataDirs(deployDir string, lightningIPs []string, dataDirs []string) (res []string, err error) {
	if len(dataDirs) == 0 {
		if len(deployDir) == 0 {
			return nil, errors.New("missing deployDir")
		}
		deployDir = strings.TrimRight(deployDir, "/")

		for range lightningIPs {
			dataDirs = append(dataDirs, deployDir+"/mydumper")
		}
	}

	if len(lightningIPs) != len(dataDirs) {
		return nil, errors.New("the count of lightningIP can not match the count of dataDir")
	}

	// trim the trailing '/'
	for i, dir := range dataDirs {
		dataDirs[i] = strings.TrimRight(dir, "/")
	}

	res = dataDirs

	return
}

/**
> rm -f /tmp/go-tpc
> wget -O /tmp/go-tpc binary_url; chmod +x /tmp/go-tpc
*/
func FetchTpcc(lightningDirs []string, lightningIPs []string, binaryURL string, skipDownloading bool) (err error) {
	errg, _ := errgroup.WithContext(context.Background())

	for i := 0; i < len(lightningIPs); i++ {
		ip := lightningIPs[i]
		dir := lightningDirs[i]

		errg.Go(func() error {
			if !skipDownloading {
				if _, _, err = runCmd("ssh", ip, `rm -f /tmp/go-tpc`); err != nil {
					return err
				}

				if _, _, err = runCmd("ssh", ip, fmt.Sprintf("wget -O /tmp/go-tpc.tar.gz %s; tar -xvf /tmp/go-tpc.tar.gz -C /tmp/; rm -f /tmp/go-tpc.tar.gz; chmod +x /tmp/go-tpc", binaryURL)); err != nil {
					return err
				}
				fmt.Println("Download go-tpc binary successfully!")
			}

			if _, _, err = runCmd("ssh", ip, fmt.Sprintf("mkdir -p %s", dir)); err != nil {
				return err
			}

			return nil
		})
	}

	return errg.Wait()
}

// DropDB drop the database if exists.
func DropDB(host string, port int, dbName string) (err error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/", "root", "", host, port)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return
	}

	_, err = db.Exec("drop database if exists " + dbName)
	if err != nil {
		return
	}

	return nil
}

/**
> /tmp/go-tpc tpcc prepare -D dbName -T threadsNum --warehouses warehouseNum --output outputDir --tables [tables]
*/
func GenCSV(lightningIPs []string, lightningDirs []string, tidbIP string, tidbPort int, dbName string, warehouse, threads int) (err error) {
	var specifiedTables []string
	switch len(lightningIPs) {
	case 1:
		// empty means generating all tables
		specifiedTables = []string{""}
	case 2:
		specifiedTables = []string{"--tables stock", "--tables order_line,customer,district,history,item,new_order,orders,warehouse"}
	case 3:
		specifiedTables = []string{"--tables stock", "--tables orders,order_line", "--tables customer,district,history,item,new_order,warehouse"}
	}

	errCh := make(chan error, 3)
	wg := &sync.WaitGroup{}
	for i, lightningIP := range lightningIPs {
		ip := lightningIP
		dir := lightningDirs[i]
		tables := specifiedTables[i]
		wg.Add(1)
		stdOutMsg := make(chan string, 40)
		defer close(stdOutMsg)
		go func() {
			for line := range stdOutMsg {
				fmt.Println(line)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err = runCmdAndGetStdOutInTime(stdOutMsg, "ssh", ip, fmt.Sprintf("cd %s; rm -rf *; "+
				"/tmp/go-tpc tpcc prepare -U root -H %s -P %d -D %s -T %d --warehouses %d --output %s %s", dir, tidbIP, tidbPort, dbName, threads, warehouse, dir, tables)); err != nil {
				if err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()
	for err = range errCh {
		return
	}

	fmt.Println("#============\ngenerate csv files finished\n#============")
	return
}

// RestoreData use ssh to start importers and lightnings.
// Will change tidb-lightning-toml as:
// [mydumper]
// no-schema = true
// strict-format = true
// [mydumper.csv]
// backslash-escape = false
// delimiter = ""
// header = false
// not-null = false
// null = "NULL"
// separator = ","
// trim-last-separator = false
func RestoreData(importerIPs []string, lightningIPs []string, deployDir string) (err error) {
	fmt.Println("#============\nstart tikv-importer\n#============")
	deployDir = strings.TrimRight(deployDir, "/")

	importerSet := make(map[string]struct{})
	for _, importerIP := range importerIPs {
		importerSet[importerIP] = struct{}{}
		if _, _, err = runCmd("ssh", importerIP, fmt.Sprintf(`sh %s`, deployDir+"/scripts/stop_importer.sh")); err != nil {
			return
		}
		if _, _, err = runCmd("ssh", importerIP, fmt.Sprintf(`sh %s`, deployDir+"/scripts/start_importer.sh")); err != nil {
			return
		}
		fmt.Println(importerIP, "ok")
	}

	fmt.Println("#============\nstart tidb-lightning\n#============")
	for _, lightningIP := range lightningIPs {
		var stdOutMsg []byte
		// Set as:
		// [mydumper]
		// no-schema = true
		// strict-format = true
		configStrictFormat := `s/^no-schema = \(\s\|\S\)\+$/no-schema = true\nstrict-format = true/;`
		if stdOutMsg, _, err = runCmd("ssh", lightningIP, fmt.Sprintf(`grep 'strict-format' %s | wc -l`, deployDir+"/conf/tidb-lightning.toml")); err != nil {
			return
		}
		if string(stdOutMsg) != "0\n" {
			configStrictFormat = `s/^no-schema = \(\s\|\S\)\+$/no-schema = true/;s/^strict-format = \(\s\|\S\)\+$/strict-format = true/;`
		}

		// set region-concurrency if there is a importer exists on the same machine with a lightning
		configRegionCon := ""
		if _, ok := importerSet[lightningIP]; ok {
			configRegionCon = fmt.Sprintf(`s/^table-concurrency/region-concurrency = %d\ntable-concurrency/;`, runtime.NumCPU()*3/4)
			if stdOutMsg, _, err = runCmd("ssh", lightningIP, fmt.Sprintf("grep 'region-concurrency' %s | wc -l", deployDir+"/conf/tidb-lightning.toml")); err != nil {
				fmt.Println("err != nil", err, string(stdOutMsg))
				return
			}
			fmt.Println(string(stdOutMsg))
			if string(stdOutMsg) != "0\n" {
				configRegionCon = fmt.Sprintf(`s/^region-concurrency = \(\s\|\S\)\+$/region-concurrency = %d/;`, runtime.NumCPU()*3/4)
			}
		}
		// Remote this for this reason:
		// This assume the machine running lightning has the same NumCPU as the machine running this tool which is wrong.
		configRegionCon = ""

		// Set as:
		// [mydumper.csv]
		// backslash-escape = false
		// delimiter = ""
		// header = false
		// not-null = false
		// null = "NULL"
		// separator = ","
		// trim-last-separator = false
		sedLightningConf := `sed -i "` +
			configRegionCon +
			configStrictFormat +
			`s/^backslash-escape = \(\s\|\S\)\+$/backslash-escape = false/;` +
			`s/^delimiter = \(\s\|\S\)\+$/delimiter = \"\"/;` +
			`s/^header = \(\s\|\S\)\+$/header = false/;` +
			`s/^not-null = \(\s\|\S\)\+$/not-null = false/;` +
			`s/^null = \(\s\|\S\)\+$/null = \"NULL\"/;` +
			`s/^separator = \(\s\|\S\)\+$/separator = \",\"/;` +
			`s/^trim-last-separator = \(\s\|\S\)\+$/trim-last-separator = false/"`
		if _, _, err = runCmd("ssh", lightningIP, fmt.Sprintf(`%s %s`, sedLightningConf, deployDir+"/conf/tidb-lightning.toml")); err != nil {
			return
		}
		if _, _, err = runCmd("ssh", lightningIP, fmt.Sprintf(`sh %s`, deployDir+"/scripts/start_lightning.sh")); err != nil {
			return
		}
		fmt.Println(lightningIP, "ok")
	}

	fmt.Println("#============\nrestore phase starts, please check the phase in lightnings' logs\n#============")
	return
}

func RunTPCCTest(lightningIP, tidbIP string, tidbPort int, dbName string, warehouse, threads int) (err error) {
	errCh := make(chan error, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	stdOutMsg := make(chan string, 40)
	defer close(stdOutMsg)
	go func() {
		for line := range stdOutMsg {
			fmt.Println(line)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err = runCmdAndGetStdOutInTime(stdOutMsg, "ssh", lightningIP, fmt.Sprintf("/tmp/go-tpc tpcc check -U root -H %s -P %d -D %s -T %d --warehouses %d", tidbIP, tidbPort, dbName, threads, warehouse)); err != nil {
			return
		}
		if _, err = runCmdAndGetStdOutInTime(stdOutMsg, "ssh", lightningIP, fmt.Sprintf("/tmp/go-tpc tpcc run --time %s -U root -H %s -P %d -D %s -T %d --warehouses %d", *tpcruntime, tidbIP, tidbPort, dbName, threads, warehouse)); err != nil {
			return
		}
		if _, err = runCmdAndGetStdOutInTime(stdOutMsg, "ssh", lightningIP, fmt.Sprintf("/tmp/go-tpc tpcc check -U root -H %s -P %d -D %s -T %d --warehouses %d", tidbIP, tidbPort, dbName, threads, warehouse)); err != nil {
			return
		}
	}()
	go func() {
		wg.Wait()
		close(errCh)
	}()
	for err = range errCh {
		return
	}

	fmt.Println("#============\ntpcc test finished\n#============")
	return
}

func runCmd(name string, arg ...string) (stdOutBytes []byte, stdErrBytes []byte, err error) {
	if _, err = exec.LookPath(name); err != nil {
		fmt.Printf("%s %s\n%s", name, strings.Join(arg, " "), err.Error())
		return
	}
	cmd := exec.Command(name, arg...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return
	}
	// todo: print to log
	fmt.Println(cmd.String())
	if err = cmd.Start(); err != nil {
		fmt.Println(err)
		return
	}
	stdOutBytes, err = ioutil.ReadAll(stdout)
	if err != nil {
		return
	}
	stdErrBytes, err = ioutil.ReadAll(stderr)
	if err != nil {
		return
	}
	if err = cmd.Wait(); err != nil {
		fmt.Printf("%s", stdErrBytes)
	}
	return
}

func runCmdAndGetStdOutInTime(stdOutMsg chan string, name string, arg ...string) (stdErrBytes []byte, err error) {
	if _, err = exec.LookPath(name); err != nil {
		fmt.Printf("%s %s\n%s", name, strings.Join(arg, " "), err.Error())
		return
	}
	cmd := exec.Command(name, arg...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return
	}
	fmt.Println(cmd.String())
	if err = cmd.Start(); err != nil {
		fmt.Println(err)
		return
	}
	reader := bufio.NewReader(stdout)
	var line string
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Println("generate data for tpcc failed, please retry")
			return
		}
		stdOutMsg <- line
	}
	stdErrBytes, err = ioutil.ReadAll(stderr)
	if err != nil {
		return
	}
	if err = cmd.Wait(); err != nil {
		fmt.Printf("%s", stdErrBytes)
	}
	return
}
