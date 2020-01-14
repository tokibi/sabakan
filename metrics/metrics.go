package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "sabakan"
)

// MachineStatus returns the machine state metrics
var MachineStatus = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "machine_status",
		Help:      "The machine status set by HTTP API.",
	},
	[]string{"status", "address", "serial", "datacenter", "rack", "index"},
)

// GetHandler return http.Handler for prometheus metrics
func GetHandler() http.Handler {
	registry := prometheus.NewRegistry()
	registerMetrics(registry)

	handler := promhttp.HandlerFor(registry,
		promhttp.HandlerOpts{
			ErrorLog:      logger{},
			ErrorHandling: promhttp.ContinueOnError,
		})

	return handler
}

func registerMetrics(registry *prometheus.Registry) {
	registry.MustRegister(MachineStatus)
	MachineStatus.WithLabelValues("unhealthy", "1.2.3.4", "123456", "moon", "0", "0").Set(1)
}
