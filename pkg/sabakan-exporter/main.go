package main

import (
	"context"
	"encoding/json"
	"flag"
	"net/http"
	"time"

	"github.com/cybozu-go/log"
	"github.com/cybozu-go/well"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	addr        = flag.String("listen-address", "http://localhost:2112", "The address to listen on for HTTP requests.")
	sabakanAddr = flag.String("sabakan-address", "http://localhost:8080", "The address of sabakan server.")

	sabakanHealth = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "sabakan",
		Subsystem: "api",
		Name:      "is_healthy",
		Help:      "Response of sabakan health API",
	})
	httpClient = &well.HTTPClient{
		Client: &http.Client{},
	}
)

func main() {
	flag.Parse()
	well.LogConfig{}.Apply()

	log.Info("listening "+*addr, map[string]interface{}{})
	prometheus.MustRegister(sabakanHealth)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	s := &well.HTTPServer{
		Server: &http.Server{
			Addr:    string((*addr)[len("http://"):]),
			Handler: mux,
		},
		ShutdownTimeout: 3 * time.Minute,
	}
	s.ListenAndServe()

	well.Go(checkSabakanHealth)
	err := well.Wait()
	if !well.IsSignaled(err) && err != nil {
		log.ErrorExit(err)
	}
}

func checkSabakanHealth(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			req, err := http.NewRequest(http.MethodGet, *sabakanAddr+"/health", nil)
			if err != nil {
				return err
			}
			res, err := httpClient.Do(req)
			if err != nil {
				log.Error("failed to health check", map[string]interface{}{
					log.FnError: err.Error(),
				})
				sabakanHealth.Set(1)
				continue
			}

			result := &struct {
				Health string `json:"health"`
			}{}
			err = json.NewDecoder(res.Body).Decode(result)
			if err != nil {
				return err
			}
			if result.Health == "healthy" {
				sabakanHealth.Set(0)
			} else {
				sabakanHealth.Set(1)
			}
		}
	}
}
