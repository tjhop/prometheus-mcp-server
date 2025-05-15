package prometheus

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	config_util "github.com/prometheus/common/config"
)

func NewAPIClient(prometheusUrl, httpConfig string) (promv1.API, error) {
	httpClient := http.DefaultClient
	if httpConfig != "" {
		httpCfg, _, err := config_util.LoadHTTPConfigFile(httpConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load HTTP configuration file %s: %w", httpConfig, err)
		}

		if err = httpCfg.Validate(); err != nil {
			return nil, fmt.Errorf("failed to validate HTTP configuration file %s: %w", httpConfig, err)
		}

		httpClient, err = config_util.NewClientFromConfig(*httpCfg, "prometheus-mcp-server")
		if err != nil {
			return nil, fmt.Errorf("failed to create HTTP client from configuration file %s: %w", httpConfig, err)
		}
	}

	client, err := api.NewClient(api.Config{
		Client:  httpClient,
		Address: prometheusUrl,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return promv1.NewAPI(client), nil
}
