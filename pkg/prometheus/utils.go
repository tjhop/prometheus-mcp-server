package prometheus

import (
	"fmt"
	"math"
	"strconv"
	"time"
)

// ParseTimestamp parses a timestamp from a string. The timestamp can be either
// a unix epoch timestamp or RFC33339 format.
func ParseTimestamp(ts string) (time.Time, error) {
	// According to API docs:
	//
	// <rfc3339 | unix_timestamp>: Input timestamps may be provided either in
	// RFC3339 format or as a Unix timestamp in seconds, with optional decimal
	// places for sub-second precision. Output timestamps are always represented
	// as Unix timestamps in seconds.
	//
	// https://prometheus.io/docs/prometheus/latest/querying/api/#format-overview

	// is it a unix timestamp?
	if unixTs, err := strconv.ParseFloat(ts, 64); err == nil {
		s, ns := math.Modf(unixTs)
		t := time.Unix(int64(s), int64(ns*float64(time.Second)))
		return t, nil
	}

	// if not a unix timestamp, is it RFC3339?
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t, nil
	}

	return time.Time{}, fmt.Errorf("failed to parse %s to timestamp", ts)
}
