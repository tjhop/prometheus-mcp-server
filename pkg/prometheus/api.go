package prometheus

import (
	"fmt"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

func NewAPIClient() (v1.API, error) {
	// TODO: allow setting basic auth/tls/etc

	client, err := api.NewClient(api.Config{
		Address: "http://127.0.0.1:9090",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return v1.NewAPI(client), nil
}
