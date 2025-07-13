package metrics

import (
	"runtime"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

const (
	MetricNamespace = "prom_mcp"
)

var (
	once     sync.Once
	Registry *prometheus.Registry
)

func init() {
	once.Do(func() {
		Registry = prometheus.NewRegistry()

		// expose build info metric
		metricBuildInfo := prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{
				Name: prometheus.BuildFQName(MetricNamespace, "build", "info"),
				Help: "A metric with a constant '1' value with labels for version, commit and build_date from which prometheus-mcp-server was built.",
				ConstLabels: prometheus.Labels{
					"version":    version.Version,
					"commit":     version.Commit,
					"build_date": version.BuildDate,
					"goversion":  runtime.Version(),
				},
			},
			func() float64 { return 1 },
		)

		Registry.MustRegister(
			// add standard process/go metrics to registry
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			collectors.NewGoCollector(),
			// register build info metric
			metricBuildInfo,
		)
	})
}
