package prometheus

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

// NewAPIClient creates a Prometheus API client configured with a custom
// User-Agent header. If rt is nil, http.DefaultTransport is used.
func NewAPIClient(prometheusURL string, rt http.RoundTripper) (promv1.API, error) {
	if rt == nil {
		rt = http.DefaultTransport
	}

	uart := config.NewUserAgentRoundTripper(version.UserAgent(), rt)

	client, err := api.NewClient(api.Config{
		Address:      prometheusURL,
		RoundTripper: uart,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return promv1.NewAPI(client), nil
}
