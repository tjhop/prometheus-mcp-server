package prometheus

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

var (
	userAgent = fmt.Sprintf("prometheus-mcp-server/%s (https://github.com/tjhop/prometheus-mcp-server)", version.Version)
)

func NewAPIClient(prometheusUrl string, rt http.RoundTripper) (promv1.API, error) {
	if rt == nil {
		rt = http.DefaultTransport
	}

	uart := config.NewUserAgentRoundTripper(userAgent, rt)

	client, err := api.NewClient(api.Config{
		Address:      prometheusUrl,
		RoundTripper: uart,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return promv1.NewAPI(client), nil
}
