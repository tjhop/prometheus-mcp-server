package mcp

import (
	"context"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

var _ promv1.API = (*MockPrometheusAPI)(nil)

// MockPrometheusAPI is a mock implementation of the promv1.API interface.
type MockPrometheusAPI struct {
	AlertManagersFunc   func(ctx context.Context) (promv1.AlertManagersResult, error)
	AlertsFunc          func(ctx context.Context) (promv1.AlertsResult, error)
	BuildinfoFunc       func(ctx context.Context) (promv1.BuildinfoResult, error)
	ConfigFunc          func(ctx context.Context) (promv1.ConfigResult, error)
	CleanTombstonesFunc func(ctx context.Context) error
	DeleteSeriesFunc    func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error
	FlagsFunc           func(ctx context.Context) (promv1.FlagsResult, error)
	LabelNamesFunc      func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]string, promv1.Warnings, error)
	LabelValuesFunc     func(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error)
	MetadataFunc        func(ctx context.Context, metric string, limit string) (map[string][]promv1.Metadata, error)
	QueryFunc           func(ctx context.Context, query string, ts time.Time, opts ...promv1.Option) (model.Value, promv1.Warnings, error)
	QueryExemplarsFunc  func(ctx context.Context, query string, startTime time.Time, endTime time.Time) ([]promv1.ExemplarQueryResult, error)
	QueryRangeFunc      func(ctx context.Context, query string, r promv1.Range, opts ...promv1.Option) (model.Value, promv1.Warnings, error)
	RulesFunc           func(ctx context.Context) (promv1.RulesResult, error)
	RuntimeinfoFunc     func(ctx context.Context) (promv1.RuntimeinfoResult, error)
	SeriesFunc          func(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]model.LabelSet, promv1.Warnings, error)
	SnapshotFunc        func(ctx context.Context, skipHead bool) (promv1.SnapshotResult, error)
	TargetsFunc         func(ctx context.Context) (promv1.TargetsResult, error)
	TargetsMetadataFunc func(ctx context.Context, matchTarget string, metric string, limit string) ([]promv1.MetricMetadata, error)
	TSDBFunc            func(ctx context.Context, opts ...promv1.Option) (promv1.TSDBResult, error)
	WalReplayFunc       func(ctx context.Context) (promv1.WalReplayStatus, error)
}

// Implement all methods of promv1.API to delegate to the function fields.
func (m *MockPrometheusAPI) AlertManagers(ctx context.Context) (promv1.AlertManagersResult, error) {
	if m.AlertManagersFunc != nil {
		return m.AlertManagersFunc(ctx)
	}
	return promv1.AlertManagersResult{}, nil
}
func (m *MockPrometheusAPI) Alerts(ctx context.Context) (promv1.AlertsResult, error) {
	if m.AlertsFunc != nil {
		return m.AlertsFunc(ctx)
	}
	return promv1.AlertsResult{}, nil
}
func (m *MockPrometheusAPI) Buildinfo(ctx context.Context) (promv1.BuildinfoResult, error) {
	if m.BuildinfoFunc != nil {
		return m.BuildinfoFunc(ctx)
	}
	return promv1.BuildinfoResult{}, nil
}
func (m *MockPrometheusAPI) Config(ctx context.Context) (promv1.ConfigResult, error) {
	if m.ConfigFunc != nil {
		return m.ConfigFunc(ctx)
	}
	return promv1.ConfigResult{}, nil
}
func (m *MockPrometheusAPI) CleanTombstones(ctx context.Context) error {
	if m.CleanTombstonesFunc != nil {
		return m.CleanTombstonesFunc(ctx)
	}
	return nil
}
func (m *MockPrometheusAPI) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	if m.DeleteSeriesFunc != nil {
		return m.DeleteSeriesFunc(ctx, matches, startTime, endTime)
	}
	return nil
}
func (m *MockPrometheusAPI) Flags(ctx context.Context) (promv1.FlagsResult, error) {
	if m.FlagsFunc != nil {
		return m.FlagsFunc(ctx)
	}
	return promv1.FlagsResult{}, nil
}
func (m *MockPrometheusAPI) LabelNames(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]string, promv1.Warnings, error) {
	if m.LabelNamesFunc != nil {
		return m.LabelNamesFunc(ctx, matches, startTime, endTime, opts...)
	}
	return nil, nil, nil
}
func (m *MockPrometheusAPI) LabelValues(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) (model.LabelValues, promv1.Warnings, error) {
	if m.LabelValuesFunc != nil {
		return m.LabelValuesFunc(ctx, label, matches, startTime, endTime, opts...)
	}
	return nil, nil, nil
}
func (m *MockPrometheusAPI) Metadata(ctx context.Context, metric string, limit string) (map[string][]promv1.Metadata, error) {
	if m.MetadataFunc != nil {
		return m.MetadataFunc(ctx, metric, limit)
	}
	return nil, nil
}
func (m *MockPrometheusAPI) Query(ctx context.Context, query string, ts time.Time, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
	if m.QueryFunc != nil {
		return m.QueryFunc(ctx, query, ts, opts...)
	}
	return nil, nil, nil
}
func (m *MockPrometheusAPI) QueryExemplars(ctx context.Context, query string, startTime time.Time, endTime time.Time) ([]promv1.ExemplarQueryResult, error) {
	if m.QueryExemplarsFunc != nil {
		return m.QueryExemplarsFunc(ctx, query, startTime, endTime)
	}
	return nil, nil
}
func (m *MockPrometheusAPI) QueryRange(ctx context.Context, query string, r promv1.Range, opts ...promv1.Option) (model.Value, promv1.Warnings, error) {
	if m.QueryRangeFunc != nil {
		return m.QueryRangeFunc(ctx, query, r, opts...)
	}
	return nil, nil, nil
}
func (m *MockPrometheusAPI) Rules(ctx context.Context) (promv1.RulesResult, error) {
	if m.RulesFunc != nil {
		return m.RulesFunc(ctx)
	}
	return promv1.RulesResult{}, nil
}
func (m *MockPrometheusAPI) Runtimeinfo(ctx context.Context) (promv1.RuntimeinfoResult, error) {
	if m.RuntimeinfoFunc != nil {
		return m.RuntimeinfoFunc(ctx)
	}
	return promv1.RuntimeinfoResult{}, nil
}
func (m *MockPrometheusAPI) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time, opts ...promv1.Option) ([]model.LabelSet, promv1.Warnings, error) {
	if m.SeriesFunc != nil {
		return m.SeriesFunc(ctx, matches, startTime, endTime, opts...)
	}
	return nil, nil, nil
}
func (m *MockPrometheusAPI) Snapshot(ctx context.Context, skipHead bool) (promv1.SnapshotResult, error) {
	if m.SnapshotFunc != nil {
		return m.SnapshotFunc(ctx, skipHead)
	}
	return promv1.SnapshotResult{}, nil
}
func (m *MockPrometheusAPI) Targets(ctx context.Context) (promv1.TargetsResult, error) {
	if m.TargetsFunc != nil {
		return m.TargetsFunc(ctx)
	}
	return promv1.TargetsResult{}, nil
}
func (m *MockPrometheusAPI) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]promv1.MetricMetadata, error) {
	if m.TargetsMetadataFunc != nil {
		return m.TargetsMetadataFunc(ctx, matchTarget, metric, limit)
	}
	return nil, nil
}
func (m *MockPrometheusAPI) TSDB(ctx context.Context, opts ...promv1.Option) (promv1.TSDBResult, error) {
	if m.TSDBFunc != nil {
		return m.TSDBFunc(ctx, opts...)
	}
	return promv1.TSDBResult{}, nil
}
func (m *MockPrometheusAPI) WalReplay(ctx context.Context) (promv1.WalReplayStatus, error) {
	if m.WalReplayFunc != nil {
		return m.WalReplayFunc(ctx)
	}
	return promv1.WalReplayStatus{}, nil
}
