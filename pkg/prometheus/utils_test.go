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
			}

			require.True(t, tc.expectedTime.Equal(got), "expected times to be equal", "expected", tc.expectedTime, "got", got)
		})
	}
}
