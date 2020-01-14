package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cybozu-go/etcdutil"
	"github.com/cybozu-go/log"
	"github.com/cybozu-go/sabakan/v2"
	"github.com/cybozu-go/sabakan/v2/dhcpd"
	"github.com/cybozu-go/sabakan/v2/metrics"
	"github.com/cybozu-go/sabakan/v2/models/etcd"
	"github.com/cybozu-go/sabakan/v2/web"
	"github.com/cybozu-go/well"
	"go.universe.tf/netboot/dhcp4"
	"sigs.k8s.io/yaml"
)

const (
	cryptsetupEnv = "SABAKAN_CRYPTSETUP"
)

var (
	flagHTTP            = flag.String("http", defaultListenHTTP, "<Listen IP>:<Port number>")
	flagMetrics         = flag.String("metrics", defaultListenMetrics, "<Listen IP>:<Port number>")
	flagDHCPBind        = flag.String("dhcp-bind", defaultDHCPBind, "bound ip addresses and port for dhcp server")
	flagIPXEPath        = flag.String("ipxe-efi-path", defaultIPXEPath, "path to ipxe.efi")
	flagDataDir         = flag.String("data-dir", defaultDataDir, "directory to store files")
	flagAdvertiseURL    = flag.String("advertise-url", "", "public URL of this server")
	flagAllowIPs        = flag.String("allow-ips", strings.Join(defaultAllowIPs, ","), "comma-separated IPs allowed to change resources")
	flagMetricsInterval = flag.String("metrics-interval", "30s", "interval duration to collect metrics data")
	flagPlayground      = flag.Bool("enable-playground", false, "enable GraphQL playground")

	flagEtcdEndpoints = flag.String("etcd-endpoints", strings.Join(etcdutil.DefaultEndpoints, ","), "comma-separated URLs of the backend etcd endpoints")
	flagEtcdPrefix    = flag.String("etcd-prefix", defaultEtcdPrefix, "etcd prefix")
	flagEtcdTimeout   = flag.String("etcd-timeout", etcdutil.DefaultTimeout, "dial timeout to etcd")
	flagEtcdUsername  = flag.String("etcd-username", "", "username for etcd authentication")
	flagEtcdPassword  = flag.String("etcd-password", "", "password for etcd authentication")
	flagEtcdTLSCA     = flag.String("etcd-tls-ca", "", "path to CA bundle used to verify certificates of etcd servers")
	flagEtcdTLSCert   = flag.String("etcd-tls-cert", "", "path to my certificate used to identify myself to etcd servers")
	flagEtcdTLSKey    = flag.String("etcd-tls-key", "", "path to my key used to identify myself to etcd servers")

	flagConfigFile = flag.String("config-file", "", "path to configuration file")
)

func main() {
	flag.Parse()
	well.LogConfig{}.Apply()

	// seed math/random
	rand.Seed(time.Now().UnixNano())

	well.Go(subMain)
	well.Stop()
	err := well.Wait()
	if !well.IsSignaled(err) && err != nil {
		log.ErrorExit(err)
	}
}

func findCryptSetup() string {
	p := os.Getenv(cryptsetupEnv)
	if p != "" {
		return p
	}
	p, err := filepath.EvalSymlinks("/proc/self/exe")
	if err != nil {
		return ""
	}
	p, err = filepath.Abs(p)
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(p), "sabakan-cryptsetup")
}

func subMain(ctx context.Context) error {
	cfg := newConfig()
	if *flagConfigFile == "" {
		cfg.AdvertiseURL = *flagAdvertiseURL
		cfg.AllowIPs = strings.Split(*flagAllowIPs, ",")
		cfg.DHCPBind = *flagDHCPBind
		cfg.DataDir = *flagDataDir
		cfg.IPXEPath = *flagIPXEPath
		cfg.ListenHTTP = *flagHTTP
		cfg.Playground = *flagPlayground
		cfg.ListenMetrics = *flagMetrics
		cfg.MetricsInterval = *flagMetricsInterval

		cfg.Etcd.Endpoints = strings.Split(*flagEtcdEndpoints, ",")
		cfg.Etcd.Prefix = *flagEtcdPrefix
		cfg.Etcd.Timeout = *flagEtcdTimeout
		cfg.Etcd.Username = *flagEtcdUsername
		cfg.Etcd.Password = *flagEtcdPassword
		cfg.Etcd.TLSCAFile = *flagEtcdTLSCA
		cfg.Etcd.TLSCertFile = *flagEtcdTLSCert
		cfg.Etcd.TLSKeyFile = *flagEtcdTLSKey
	} else {
		data, err := ioutil.ReadFile(*flagConfigFile)
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(data, cfg)
		if err != nil {
			return err
		}
	}

	if !filepath.IsAbs(cfg.DataDir) {
		return errors.New("data-dir must be an absolute path")
	}
	if cfg.AdvertiseURL == "" {
		return errors.New("advertise-url must be specified")
	}
	advertiseURL, err := url.Parse(cfg.AdvertiseURL)
	if err != nil {
		return err
	}

	c, err := etcdutil.NewClient(cfg.Etcd)
	if err != nil {
		return err
	}
	defer c.Close()

	model := etcd.NewModel(c, cfg.DataDir, advertiseURL)

	// update schema
	sv, err := model.Schema.Version(ctx)
	if err != nil {
		return err
	}
	if sv != sabakan.SchemaVersion {
		err = model.Schema.Upgrade(ctx)
		if err != nil {
			return err
		}
	}

	env := well.NewEnvironment(ctx)
	ch := make(chan struct{})
	env.Go(func(ctx context.Context) error {
		return model.Run(ctx, ch)
	})

	// waiting the driver gets ready
	<-ch

	conn, err := dhcp4.NewConn(cfg.DHCPBind)
	if err != nil {
		return err
	}
	dhcpServer := dhcpd.Server{
		Handler: dhcpd.DHCPHandler{Model: model, MyURL: advertiseURL},
		Conn:    conn,
	}
	env.Go(dhcpServer.Serve)

	cryptsetupPath := findCryptSetup()
	allowedIPs, err := parseAllowIPs(cfg.AllowIPs)
	if err != nil {
		return err
	}
	webServer := web.NewServer(model, cfg.IPXEPath, cryptsetupPath, advertiseURL, allowedIPs, cfg.Playground)
	s := &well.HTTPServer{
		Server: &http.Server{
			Addr:    cfg.ListenHTTP,
			Handler: webServer,
		},
		ShutdownTimeout: 3 * time.Minute,
		Env:             env,
	}
	s.ListenAndServe()

	interval, err := time.ParseDuration(cfg.MetricsInterval)
	if err != nil {
		return fmt.Errorf("unable to parse duration; %w", err)
	}
	metricsUpdater := metrics.NewUpdater(interval, &model)
	env.Go(metricsUpdater.UpdateLoop)

	metricsHandler := metrics.GetHandler()
	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsHandler)
	ms := &well.HTTPServer{
		Server: &http.Server{
			Addr:    cfg.ListenMetrics,
			Handler: mux,
		},
	}
	ms.ListenAndServe()

	env.Stop()
	return env.Wait()
}

func parseAllowIPs(ips []string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, len(ips))
	for i, cidr := range ips {
		if !strings.Contains(cidr, "/") {
			cidr += "/32"
		}
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, err
		}
		nets[i] = ipnet
	}
	return nets, nil
}
