package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	nmAll     = "all"
	nmCSV     = "csv"
	nmRestore = "restore"

	nmWarehouse = "warehouse"

	nmTiDBIP    = "tidb-ip"
	nmTiDBPort  = "tidb-port"
	nmDeployDir = "deploy-dir"

	//nmAnsibleDir    = "ansible-dir"
	nmLightningIP = "lightning-ip"
	nmDataDir     = "data-dir"

	nmImporterIP = "importer-ip"
)

var (
	all     = flag.Bool(nmAll, true, "do all the actions")
	csv     = flag.Bool(nmCSV, false, "generate tpcc csv files")
	restore = flag.Bool(nmRestore, false, "start lightning, importer and restore files")

	tidbIP    = flag.String(nmTiDBIP, "127.0.0.1", "ip of tidb-server")
	tidbPort  = flag.String(nmTiDBPort, "4000", "port of tidb-server")
	deployDir = flag.String(nmDeployDir, "", "directory path of cluster deployment")

	warehouse = flag.Int64(nmWarehouse, 100, "count of warehouse")

	//ansibleDir = flag.String(nmAnsibleDir, "", "ansible directory path")

	// TODO: If there is only one lightning, we do not need this var, we can fetch it from ansible.
	lightningIP = flag.String(nmLightningIP, "", "ip address of tidb-lightnings")
	dataDir     = flag.String(nmDataDir, "", "data source directory of lightning")
	importerIP  = flag.String(nmImporterIP, "", "ip address of tikv-importer")
)

func main() {
	start := time.Now()
	flag.Parse()
	if *csv || *restore {
		*all = false
	}

	lightningIPs, dataDirs, err := getLightningIPsAndDataDirs()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	var start2 time.Time
	if *all || *csv {
		if err = fetchTpccRepoAndEnforceConf(*tidbIP, *tidbPort, *warehouse, dataDirs, lightningIPs); err != nil {
			os.Exit(1)
		}
		start2 = time.Now()
		if err = genSchema(*tidbIP, *tidbPort, lightningIPs[0]); err != nil {
			os.Exit(1)
		}
		if err = genCSV(lightningIPs, dataDirs); err != nil {
			os.Exit(1)
		}
	}
	fmt.Println("prepare cost:", time.Since(start).String(), "gen data cost:", time.Since(start2).String())

	if len(*importerIP) == 0 {
		fmt.Println("missing importerIP")
		os.Exit(1)
	}
	importerIPs := strings.Split(*importerIP, ",")
	if len(importerIPs) != len(lightningIPs) {
		fmt.Println("the count of importerIP mismatch the count of lightningIP")
		os.Exit(1)
	}
	if len(*deployDir) == 0 {
		fmt.Println("missing deployDir")
		os.Exit(1)
	}
	if *all || *restore {
		if strings.LastIndex(*deployDir, "/") == len(*deployDir)-1 {
			*deployDir = string([]byte(*deployDir)[0 : len(*deployDir)-1])
		}
		if err = restoreData(importerIPs, lightningIPs, *deployDir); err != nil {
			os.Exit(1)
		}
	}
	os.Exit(0)

}

func getLightningIPsAndDataDirs() (lightningIPs, dataDirs []string, err error) {
	// todo: fetch ansible inventory.ini, get tidbIp tidbPort, deployDir
	if len(*lightningIP) == 0 {
		return nil, nil, errors.New("missing lightningIP")
	}
	lightningIPs = strings.Split(*lightningIP, ",")
	if len(*dataDir) == 0 {
		if len(*deployDir) == 0 {
			return nil, nil, errors.New("missing deployDir")
		}
		if strings.LastIndex(*deployDir, "/") == len(*deployDir)-1 {
			*deployDir = string([]byte(*deployDir)[0 : len(*deployDir)-1])
		}
		for range lightningIPs {
			dataDirs = append(dataDirs, *deployDir+"/mydumper")
		}
	} else {
		dataDirs = strings.Split(*dataDir, ",")
		if len(lightningIPs) != len(dataDirs) {
			return nil, nil, errors.New("the count of lightningIP can not match the count of dataDir")
		}
		// trim the trailing '/'
		for i, dir := range dataDirs {
			if strings.LastIndex(dir, "/") == len(dir)-1 {
				dataDirs[i] = string([]byte(dir)[0 : len(dir)-1])
			}
		}
	}

	if len(lightningIPs) > 3 {
		lightningIPs = lightningIPs[:3]
		dataDirs = dataDirs[:3]
	}

	return
}

/**
> rm -rf /tmp/benchmarksql
> git clone https://github.com/pingcap/benchmarksql.git /tmp/benchmarksql
> yum install -y java ant
> cd /tmp/benchmarksql
> ant
> sed -i 's/localhost:4000/tidb_ip:tidb_port' /tmp/benchmarksql/run/props.mysql
> sed -i "s/warehouses=[0-9]\+/warehouses=10000/" /tmp/benchmarksql/run/props.mysql
> echo fileLocation=%s >> /tmp/benchmarksql/run/props.mysql
> echo tableName=%s >> /tmp/benchmarksql/run/props.mysql
*/
func fetchTpccRepoAndEnforceConf(tidbIP, tidbPort string, warehouse int64, lightningDirs []string, lightningIPs []string) (err error) {
	var tableName []string
	switch len(lightningIPs) {
	case 1:
		tableName = []string{"all"}
	case 2:
		tableName = []string{"customer", "stock,order"}
	case 3:
		tableName = []string{"customer", "stock", "order"}
	}

	errCh := make(chan error, 3)
	wg := &sync.WaitGroup{}
	for i, lightningIP := range lightningIPs {
		tn := tableName[i]
		ip := lightningIP
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err = runCmd("ssh", ip, `rm -rf /tmp/benchmarksql`); err != nil {
				errCh <- err
				return
			}
			if _, _, err = runCmd("ssh", ip, `cd /tmp; git clone -b specify_table https://github.com/XuHuaiyu/benchmarksql.git`); err != nil {
				errCh <- err
				return
			}
			if _, _, err = runCmd("ssh", ip, `sudo yum install -y java ant`); err != nil {
				errCh <- err
				return
			}
			if _, _, err = runCmd("ssh", ip, fmt.Sprintf(`cd /tmp/benchmarksql; ant`)); err != nil {
				errCh <- err
				return
			}
			if _, _, err = runCmd("ssh", ip, fmt.Sprintf(`sed -i '%s' %s`, fmt.Sprintf(`s/localhost:4000/%s/;s/warehouses=[0-9]\+/%s/;s/loadWorkers=[0-9]\+/loadWorkers=%d/`, tidbIP+":"+tidbPort, fmt.Sprintf("warehouses=%d", warehouse), runtime.NumCPU()), "/tmp/benchmarksql/run/props.mysql")); err != nil {
				errCh <- err
				return
			}
			if _, _, err = runCmd("ssh", ip, fmt.Sprintf("echo fileLocation=%s/tpcc. >> /tmp/benchmarksql/run/props.mysql", lightningDirs[i])); err != nil {
				errCh <- err
				return
			}
			if _, _, err = runCmd("ssh", ip, fmt.Sprintf("echo tableName=%s >> /tmp/benchmarksql/run/props.mysql", tn)); err != nil {
				errCh <- err
				return
			}
			if _, _, err = runCmd("ssh", ip, fmt.Sprintf("mkdir -p %s", lightningDirs[i])); err != nil {
				errCh <- err
				return
			}
		}()
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()
	for err = range errCh {
		if err != nil {
			return
		}
	}

	return
}

/**
> mysql -h tidbIP -u root -P tidbPort -e "drop database if exists tpcc"
> mysql -h tidbIP -u root -P tidbPort -e "create database tpcc"`
> cd /tmp/benchmarksql/run
> ./runSQL.sh props.mysql sql.mysql/tableCreates.sql
> ./runSQL.sh props.mysql sql.mysql/indexCreates.sql
> cd -
*/
func genSchema(tidbIP, tidbPort string, lightningIP string) (err error) {
	if _, _, err = runCmd("bash", "-c", fmt.Sprintf(`mysql -h %s -u root -P %s -e "drop database if exists tpcc"`, tidbIP, tidbPort)); err != nil {
		return
	}
	if _, _, err = runCmd("bash", "-c", fmt.Sprintf(`mysql -h %s -u root -P %s -e "create database tpcc"`, tidbIP, tidbPort)); err != nil {
		return
	}
	var stdOutMsg []byte
	if stdOutMsg, _, err = runCmd("ssh", lightningIP, "cd /tmp/benchmarksql/run; ./runSQL.sh props.mysql sql.mysql/tableCreates.sql; ./runSQL.sh props.mysql sql.mysql/indexCreates.sql"); err != nil {
		return
	}
	fmt.Printf("%s", stdOutMsg)
	return
}

/**
> cd /tmp/benchmarksql/run
> ./runLoader.sh props.mysql props.mysql
*/
func genCSV(lightningIPs []string, lightningDirs []string) (err error) {
	errCh := make(chan error, 3)
	wg := &sync.WaitGroup{}
	for i, lightningIP := range lightningIPs {
		ip := lightningIP
		dir := lightningDirs[i]
		wg.Add(1)
		stdOutMsg := make(chan string, 40)
		go func() {
			for line := range stdOutMsg {
				fmt.Println(line)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err = runCmdAndGetStdOutInTime(stdOutMsg, "ssh", ip, fmt.Sprintf("cd %s; rm -rf *; cd /tmp/benchmarksql/run; ./runLoader.sh props.mysql props.mysql", dir)); err != nil {
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

// use ssh to start importers and lightnings.
func restoreData(importerIPs []string, lightningIPs []string, deployDir string) (err error) {
	fmt.Println("#============\nstart tikv-importer\n#============")
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
		configStrictFormat := `s/^no-schema = \(\s\|\S\)\+$/no-schema = true\nstrict-format = true/;`
		if stdOutMsg, _, err = runCmd("ssh", lightningIP, fmt.Sprintf(`grep 'strict-format' %s | wc -l`, deployDir+"/conf/tidb-lightning.toml")); err != nil {
			return
		}
		if string(stdOutMsg) != "0\n" {
			configStrictFormat = `s/^no-schema = \(\s\|\S\)\+$/no-schema = true/;s/^strict-format = \(\s\|\S\)\+$/strict-format = true/;`
		}
		configRegionCon := ""
		// set region-concurrency if there is a importer exists on the same machine with a lightning
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
	defer close(stdOutMsg)
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
