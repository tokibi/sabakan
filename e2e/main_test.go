package e2e

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

const (
	etcdClientURL = "http://localhost:12379"
	etcdPeerURL   = "http://localhost:12380"
)

var circleci = false

func init() {
	circleci = os.Getenv("CIRCLECI") == "true"
}

func testMain(m *testing.M) (int, error) {
	stopEtcd := runEtcd()
	defer func() {
		stopEtcd()
	}()

	stopSabakan, err := runSabakan()
	if err != nil {
		return 0, err
	}
	defer func() {
		stopSabakan()
	}()

	return m.Run(), nil
}

func runEtcd() func() {
	etcdPath, err := ioutil.TempDir("", "sabakan-test")
	if err != nil {
		log.Fatal(err)
	}
	command := exec.Command("etcd",
		"--data-dir", etcdPath,
		"--initial-cluster", "default="+etcdPeerURL,
		"--listen-peer-urls", etcdPeerURL,
		"--initial-advertise-peer-urls", etcdPeerURL,
		"--listen-client-urls", etcdClientURL,
		"--advertise-client-urls", etcdClientURL)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Start()
	if err != nil {
		log.Fatal(err)
	}

	return func() {
		command.Process.Kill()
		command.Wait()
		os.RemoveAll(etcdPath)
	}
}

func TestMain(m *testing.M) {
	if circleci {
		code := m.Run()
		os.Exit(code)
	}

	if len(os.Getenv("RUN_E2E")) == 0 {
		os.Exit(0)
	}

	status, err := testMain(m)
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(status)
}

func runSabakan() (func(), error) {
	dataDir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}

	command := exec.Command("../sabakan",
		"-dhcp-bind", "0.0.0.0:10067",
		"-etcd-servers", etcdClientURL,
		"-advertise-url", "http://localhost:10080",
		"-data-dir", dataDir,
	)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	err = command.Start()
	if err != nil {
		return nil, err
	}

	// wait for startup
	for i := 0; i < 10; i++ {
		var resp *http.Response
		resp, err = http.Get("http://localhost:10080/api/v1/config/ipam")
		if err == nil {
			resp.Body.Close()
			return func() {
				command.Process.Kill()
				command.Wait()
				os.RemoveAll(dataDir)
			}, nil
		}
		time.Sleep(1 * time.Second)
	}

	return nil, err
}

func runSabactl(args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	command := exec.Command("../sabactl", args...)
	command.Stdout = stdout
	command.Stderr = stderr
	return stdout, stderr, command.Run()
}
