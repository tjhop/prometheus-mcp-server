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

package prometheus

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/config"
	promversion "github.com/prometheus/common/version"
)

// UserAgent returns the User-Agent string used by this project's HTTP clients.
// It is a function rather than a package-level variable because the version is
// populated by the linker at build time and may not be set during var init.
func UserAgent() string {
	return fmt.Sprintf("prometheus-mcp/%s (https://github.com/prometheus/prometheus-mcp)", promversion.Version)
}

// NewAPIClient creates a Prometheus API client configured with a custom
// User-Agent header. If rt is nil, http.DefaultTransport is used.
func NewAPIClient(prometheusURL string, rt http.RoundTripper) (promv1.API, error) {
	if rt == nil {
		rt = http.DefaultTransport
	}

	uart := config.NewUserAgentRoundTripper(UserAgent(), rt)

	client, err := api.NewClient(api.Config{
		Address:      prometheusURL,
		RoundTripper: uart,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return promv1.NewAPI(client), nil
}
