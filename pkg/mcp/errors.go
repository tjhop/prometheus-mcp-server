package mcp

import (
	"errors"
	"fmt"
	"net/http"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// ErrEndpointNotSupported indicates that a Prometheus API endpoint returned
// HTTP 404, which typically means the endpoint is not available on the running
// version of Prometheus. This wraps the original error for unwrapping.
type ErrEndpointNotSupported struct {
	Endpoint   string
	StatusCode int
	Err        error
}

// Error returns a user-friendly message explaining the 404 and suggesting
// version-related remediation.
func (e *ErrEndpointNotSupported) Error() string {
	return fmt.Sprintf(
		"the API endpoint %q returned HTTP %d (%s) -- "+
			"this endpoint may not be supported by your version of Prometheus. "+
			"Consider upgrading Prometheus or using the build_info tool to check documentation for the running version.",
		e.Endpoint,
		e.StatusCode,
		http.StatusText(e.StatusCode),
	)
}

// Unwrap returns the underlying error for errors.Is/As compatibility.
func (e *ErrEndpointNotSupported) Unwrap() error {
	return e.Err
}

// isNotFoundError checks whether an error returned by the prometheus
// client_golang API client represents an HTTP 404 Not Found response.
//
// client_golang constructs 4xx error messages as "client error: <status_code>"
// (see the ErrorType/Error types in prometheus/client_golang). We use exact
// equality rather than substring matching to avoid false positives from
// messages that happen to contain "404" (e.g. "client error: 4040").
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var promErr *promv1.Error
	if !errors.As(err, &promErr) {
		return false
	}

	return promErr.Type == promv1.ErrClient && promErr.Msg == "client error: 404"
}

// wrapErrorIfNotFound wraps a 404 error from client_golang into
// ErrEndpointNotSupported with the given endpoint path. Non-404 errors are
// returned unchanged.
func wrapErrorIfNotFound(err error, endpoint string) error {
	if isNotFoundError(err) {
		return &ErrEndpointNotSupported{
			Endpoint:   endpoint,
			StatusCode: http.StatusNotFound,
			Err:        err,
		}
	}
	return err
}
