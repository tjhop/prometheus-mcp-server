package prometheus

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

var (
	// Values here are copied and un-exported from Prometheus codebase:
	// https://github.com/prometheus/prometheus/blob/main/web/api/v1/api.go#L884-L905

	// minTime is the default timestamp used for the start of optional time ranges.
	// Exposed to let downstream projects reference it.
	//
	// Historical note: This should just be time.Unix(math.MinInt64/1000, 0).UTC(),
	// but it was set to a higher value in the past due to a misunderstanding.
	// The value is still low enough for practical purposes, so we don't want
	// to change it now, avoiding confusion for importers of this variable.
	minTime = time.Unix(math.MinInt64/1000+62135596801, 0).UTC()

	// maxTime is the default timestamp used for the end of optional time ranges.
	// Exposed to let downstream projects to reference it.
	//
	// Historical note: This should just be time.Unix(math.MaxInt64/1000, 0).UTC(),
	// but it was set to a lower value in the past due to a misunderstanding.
	// The value is still high enough for practical purposes, so we don't want
	// to change it now, avoiding confusion for importers of this variable.
	maxTime = time.Unix(math.MaxInt64/1000-62135596801, 999999999).UTC()

	minTimeFormatted = minTime.Format(time.RFC3339Nano)
	maxTimeFormatted = maxTime.Format(time.RFC3339Nano)
)

// ParseTimestamp parses a timestamp from a string. The timestamp can be either
// a unix epoch timestamp or RFC33339 format.
//
// Copied from Prometheus codebase:
// https://github.com/prometheus/prometheus/blob/main/web/api/v1/api.go#L2082-L2103
func ParseTimestamp(s string) (time.Time, error) {
	if t, err := strconv.ParseFloat(s, 64); err == nil {
		s, ns := math.Modf(t)
		ns = math.Round(ns*1000) / 1000
		return time.Unix(int64(s), int64(ns*float64(time.Second))).UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}

	// Stdlib's time parser can only handle 4 digit years. As a workaround until
	// that is fixed we want to at least support our own boundary times.
	// Context: https://github.com/prometheus/client_golang/issues/614
	// Upstream issue: https://github.com/golang/go/issues/20555
	switch s {
	case minTimeFormatted:
		return minTime, nil
	case maxTimeFormatted:
		return maxTime, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse %q to a valid timestamp", s)
}
