package mcp

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/stretchr/testify/require"
)

func TestIsNotFoundError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("something went wrong"),
			expected: false,
		},
		{
			name: "promv1 ErrClient 404",
			err: &promv1.Error{
				Type: promv1.ErrClient,
				Msg:  "client error: 404",
			},
			expected: true,
		},
		{
			name: "promv1 ErrClient non-404",
			err: &promv1.Error{
				Type: promv1.ErrClient,
				Msg:  "client error: 400",
			},
			expected: false,
		},
		{
			name: "promv1 ErrServer",
			err: &promv1.Error{
				Type: promv1.ErrServer,
				Msg:  "server error: 500",
			},
			expected: false,
		},
		{
			name: "promv1 ErrBadResponse",
			err: &promv1.Error{
				Type: promv1.ErrBadResponse,
				Msg:  "bad response code 404",
			},
			expected: false,
		},
		{
			name: "wrapped promv1 ErrClient 404",
			err: fmt.Errorf("outer: %w", &promv1.Error{
				Type: promv1.ErrClient,
				Msg:  "client error: 404",
			}),
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := isNotFoundError(tc.err)
			require.Equal(t, tc.expected, result)
		})
	}
}

func TestWrapNotFoundError(t *testing.T) {
	t.Parallel()

	t.Run("wraps 404 error into ErrEndpointNotSupported", func(t *testing.T) {
		t.Parallel()

		origErr := &promv1.Error{
			Type: promv1.ErrClient,
			Msg:  "client error: 404",
		}
		endpoint := "/api/v1/status/tsdb/blocks"

		wrapped := wrapErrorIfNotFound(origErr, endpoint)

		var notSupported *ErrEndpointNotSupported
		require.ErrorAs(t, wrapped, &notSupported)
		require.Equal(t, endpoint, notSupported.Endpoint)
		require.Equal(t, http.StatusNotFound, notSupported.StatusCode)
		require.ErrorIs(t, wrapped, origErr)
	})

	t.Run("passes through non-404 error unchanged", func(t *testing.T) {
		t.Parallel()

		origErr := &promv1.Error{
			Type: promv1.ErrServer,
			Msg:  "server error: 500",
		}
		endpoint := "/api/v1/status/tsdb/blocks"

		wrapped := wrapErrorIfNotFound(origErr, endpoint)

		require.Equal(t, origErr, wrapped)
	})

	t.Run("passes through generic error unchanged", func(t *testing.T) {
		t.Parallel()

		origErr := errors.New("connection refused")
		endpoint := "/api/v1/status/tsdb/blocks"

		wrapped := wrapErrorIfNotFound(origErr, endpoint)

		require.Equal(t, origErr, wrapped)
	})
}

func TestErrEndpointNotSupportedError(t *testing.T) {
	t.Parallel()

	err := &ErrEndpointNotSupported{
		Endpoint:   "/api/v1/status/tsdb/blocks",
		StatusCode: http.StatusNotFound,
	}

	msg := err.Error()
	require.Contains(t, msg, "/api/v1/status/tsdb/blocks")
	require.Contains(t, msg, "404")
	require.Contains(t, msg, "Not Found")
	require.Contains(t, msg, "may not be supported by your version of Prometheus")
	require.Contains(t, msg, "build_info")
}
