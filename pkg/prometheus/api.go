package prometheus

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"

	"github.com/tjhop/prometheus-mcp-server/internal/version"
)

var (
	userAgent = fmt.Sprintf("prometheus-mcp-server/%s (https://github.com/tjhop/prometheus-mcp-server)", version.Version)
)

type userAgentRoundTripper struct {
	name string
	rt   http.RoundTripper
}

// RoundTrip implements the http.RoundTripper interface.
func (u userAgentRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.UserAgent() == "" {
		// The specification of http.RoundTripper says that it shouldn't mutate
		// the request so make a copy of req.Header since this is all that is
		// modified.
		r2 := new(http.Request)
		*r2 = *r
		r2.Header = make(http.Header)
		for k, s := range r.Header {
			r2.Header[k] = s
		}
		r2.Header.Set("User-Agent", u.name)
		r = r2
	}
	return u.rt.RoundTrip(r)
}

func NewAPIClient(prometheusUrl string, rt http.RoundTripper) (promv1.API, error) {
	if rt == nil {
		rt = http.DefaultTransport
	}

	uart := userAgentRoundTripper{
		name: userAgent,
		rt:   rt,
	}

	client, err := api.NewClient(api.Config{
		Address:      prometheusUrl,
		RoundTripper: uart,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return promv1.NewAPI(client), nil
}
