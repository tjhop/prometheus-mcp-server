package prometheus

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseTimestamp(t *testing.T) {
	testCases := []struct {
		name          string
		ts            string
		expectedTime  time.Time
		expectedError bool
	}{
		{
			name:          "Unix Timestamp",
			ts:            "1136214245",
			expectedTime:  time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "Unix Timestamp with Fractional Seconds",
			ts:            "1136214245.123",
			expectedTime:  time.Date(2006, 1, 2, 15, 4, 5, 123000000, time.UTC),
			expectedError: false,
		},
		{
			name:          "RFC3339 Timestamp",
			ts:            "2006-01-02T15:04:05Z",
			expectedTime:  time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
			expectedError: false,
		},
		{
			name:          "RFC3339Nano Timestamp",
			ts:            "2006-01-02T15:04:05.123Z",
			expectedTime:  time.Date(2006, 1, 2, 15, 4, 5, 123000000, time.UTC),
			expectedError: false,
		},
		{
			name:          "Invalid Timestamp",
			ts:            "invalid",
			expectedTime:  time.Time{},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseTimestamp(tc.ts)
			switch tc.expectedError {
			case true:
				require.Error(t, err)
			default:
				require.NoError(t, err)
				require.True(t, tc.expectedTime.Equal(got), "expected times to be equal", "expected", tc.expectedTime, "got", got)
			}
		})
	}
}

func TestParseTimestampOrDuration(t *testing.T) {
	// Verify timestamp input falls through to ParseTimestamp.
	t.Run("Timestamp fallthrough", func(t *testing.T) {
		got, err := ParseTimestampOrDuration("1136214245")
		require.NoError(t, err)
		expected := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
		require.True(t, expected.Equal(got), "expected times to be equal", "expected", expected, "got", got)
	})

	// Test cases for duration parsing (relative to now).
	durationCases := []struct {
		name     string
		input    string
		duration time.Duration
	}{
		{
			name:     "5 minutes",
			input:    "5m",
			duration: 5 * time.Minute,
		},
		{
			name:     "1 hour",
			input:    "1h",
			duration: 1 * time.Hour,
		},
		{
			name:     "1 hour 30 minutes",
			input:    "1h30m",
			duration: 1*time.Hour + 30*time.Minute,
		},
		{
			name:     "30 seconds",
			input:    "30s",
			duration: 30 * time.Second,
		},
	}

	for _, tc := range durationCases {
		t.Run(tc.name, func(t *testing.T) {
			before := time.Now()
			got, err := ParseTimestampOrDuration(tc.input)
			after := time.Now()

			require.NoError(t, err)

			// The result should be approximately (now - duration).
			// Account for time elapsed during the test by checking
			// the result is within the valid window.
			expectedEarliest := before.Add(-tc.duration)
			expectedLatest := after.Add(-tc.duration)

			require.False(t, got.Before(expectedEarliest),
				"result %v should not be before %v (expected earliest)", got, expectedEarliest)
			require.False(t, got.After(expectedLatest),
				"result %v should not be after %v (expected latest)", got, expectedLatest)
		})
	}

	// Test invalid input.
	t.Run("Invalid input", func(t *testing.T) {
		_, err := ParseTimestampOrDuration("not-a-timestamp-or-duration")
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot parse")
	})
}
