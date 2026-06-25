// Copyright The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

import (
	"runtime"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	promversion "github.com/prometheus/common/version"
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
				Help: "A metric with a constant '1' value with labels for version, commit and build_date from which prometheus-mcp was built.",
				ConstLabels: prometheus.Labels{
					"version":    promversion.Version,
					"commit":     promversion.Revision,
					"build_date": promversion.BuildDate,
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
