package prometheus

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testPrometheusUrl = "https://prometheus.demo.prometheus.io:443/"
)

// MockRoundTripper is a mock implementation of http.RoundTripper for testing.
type MockRoundTripper struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.RoundTripFunc != nil {
		return m.RoundTripFunc(req)
	}
	return &http.Response{}, nil
}

func TestNewAPIClient(t *testing.T) {
	testCases := []struct {
		name          string
		prometheusUrl string
		rt            http.RoundTripper
		expectedError bool
	}{
		{
			name:          "success",
			prometheusUrl: testPrometheusUrl,
			rt:            http.DefaultTransport,
			expectedError: false,
		},

		{
			name:          "with custom roundtripper",
			prometheusUrl: testPrometheusUrl,
			rt:            &MockRoundTripper{},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewAPIClient(tc.prometheusUrl, tc.rt)
			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
