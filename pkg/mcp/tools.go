package mcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ptr returns a pointer to the given value.
//
// Needed mostly since `true` is a constant and we can't take the address of it
// directly for things that need a pointer to a bool.
func ptr[T any](v T) *T {
	return &v
}

// Tool definitions for the MCP server.
var (
	queryToolDef = &mcp.Tool{
		Name:        "query",
		Description: "Execute an instant query against the Prometheus datasource",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Instant Query",
			ReadOnlyHint: true,
		},
	}

	rangeQueryToolDef = &mcp.Tool{
		Name:        "range_query",
		Description: "Execute a range query against the Prometheus datasource",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Range Query",
			ReadOnlyHint: true,
		},
	}

	exemplarQueryToolDef = &mcp.Tool{
		Name:        "exemplar_query",
		Description: "Execute a exemplar query against the Prometheus datasource",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Exemplar Query",
			ReadOnlyHint: true,
		},
	}

	seriesToolDef = &mcp.Tool{
		Name:        "series",
		Description: "Finds series by label matches",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Find Series",
			ReadOnlyHint: true,
		},
	}

	labelNamesToolDef = &mcp.Tool{
		Name:        "label_names",
		Description: "Returns the unique label names present in the block in sorted order by given time range and matches",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Label Names",
			ReadOnlyHint: true,
		},
	}

	labelValuesToolDef = &mcp.Tool{
		Name:        "label_values",
		Description: "Performs a query for the values of the given label, time range and matches",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Label Values",
			ReadOnlyHint: true,
		},
	}

	metricMetadataToolDef = &mcp.Tool{
		Name:        "metric_metadata",
		Description: "Returns metadata about metrics currently scraped by the metric name.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Metric Metadata",
			ReadOnlyHint: true,
		},
	}

	targetsMetadataToolDef = &mcp.Tool{
		Name:        "targets_metadata",
		Description: "Returns metadata about metrics currently scraped by the target ",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Targets Metadata",
			ReadOnlyHint: true,
		},
	}

	alertmanagersToolDef = &mcp.Tool{
		Name:        "alertmanagers",
		Description: "Get overview of Prometheus Alertmanager discovery",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Alertmanagers",
			ReadOnlyHint: true,
		},
	}

	flagsToolDef = &mcp.Tool{
		Name:        "flags",
		Description: "Get runtime flags",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Runtime Flags",
			ReadOnlyHint: true,
		},
	}

	listAlertsToolDef = &mcp.Tool{
		Name:        "list_alerts",
		Description: "List all active alerts",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Alerts",
			ReadOnlyHint: true,
		},
	}

	tsdbStatsToolDef = &mcp.Tool{
		Name:        "tsdb_stats",
		Description: "Get usage and cardinality statistics from the TSDB",
		Annotations: &mcp.ToolAnnotations{
			Title:        "TSDB Stats",
			ReadOnlyHint: true,
		},
	}

	buildInfoToolDef = &mcp.Tool{
		Name:        "build_info",
		Description: "Get Prometheus build information",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Build Info",
			ReadOnlyHint: true,
		},
	}

	configToolDef = &mcp.Tool{
		Name:        "config",
		Description: "Get Prometheus configuration",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Configuration",
			ReadOnlyHint: true,
		},
	}

	runtimeInfoToolDef = &mcp.Tool{
		Name:        "runtime_info",
		Description: "Get Prometheus runtime information",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Runtime Info",
			ReadOnlyHint: true,
		},
	}

	listRulesToolDef = &mcp.Tool{
		Name:        "list_rules",
		Description: "List all alerting and recording rules that are loaded",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Rules",
			ReadOnlyHint: true,
		},
	}

	listTargetsToolDef = &mcp.Tool{
		Name:        "list_targets",
		Description: "Get overview of Prometheus target discovery",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Targets",
			ReadOnlyHint: true,
		},
	}

	walReplayToolDef = &mcp.Tool{
		Name:        "wal_replay_status",
		Description: "Get current WAL replay status",
		Annotations: &mcp.ToolAnnotations{
			Title:        "WAL Replay Status",
			ReadOnlyHint: true,
		},
	}

	// TSDB Admin tools.
	cleanTombstonesToolDef = &mcp.Tool{
		Name:        "clean_tombstones",
		Description: "Removes the deleted data from disk and cleans up the existing tombstones",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Clean Tombstones",
			DestructiveHint: ptr(true),
		},
	}

	deleteSeriesToolDef = &mcp.Tool{
		Name:        "delete_series",
		Description: "Deletes data for a selection of series in a time range",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Delete Series",
			DestructiveHint: ptr(true),
		},
	}

	snapshotToolDef = &mcp.Tool{
		Name:        "snapshot",
		Description: "creates a snapshot of all current data into snapshots/<datetime>-<rand> under the TSDB's data directory and returns the directory as response.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Create Snapshot",
			DestructiveHint: ptr(true),
		},
	}

	// Management API tools.
	healthyToolDef = &mcp.Tool{
		Name:        "healthy",
		Description: "Management API endpoint that can be used to check Prometheus health.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Health Check",
			ReadOnlyHint: true,
		},
	}

	readyToolDef = &mcp.Tool{
		Name:        "ready",
		Description: "Management API endpoint that can be used to check Prometheus is ready to serve traffic (i.e. respond to queries.)",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Readiness Check",
			ReadOnlyHint: true,
		},
	}

	reloadToolDef = &mcp.Tool{
		Name:        "reload",
		Description: "Management API endpoint that can be used to trigger a reload of the Prometheus configuration and rule files.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Reload Config",
			DestructiveHint: ptr(true),
		},
	}

	quitToolDef = &mcp.Tool{
		Name:        "quit",
		Description: "Management API endpoint that can be used to trigger a graceful shutdown of Prometheus.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Shutdown",
			DestructiveHint: ptr(true),
		},
	}

	// Documentation tools.
	docsListToolDef = &mcp.Tool{
		Name:        "docs_list",
		Description: "List of Official Prometheus Documentation Files.",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Documentation",
			ReadOnlyHint: true,
		},
	}

	docsReadToolDef = &mcp.Tool{
		Name:        "docs_read",
		Description: "Read the named markdown file containing official Prometheus documentation from the prometheus/docs repo",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Read Documentation",
			ReadOnlyHint: true,
		},
	}

	docsSearchToolDef = &mcp.Tool{
		Name:        "docs_search",
		Description: "Search the markdown files containing official Prometheus documentation from the prometheus/docs repo",
		Annotations: &mcp.ToolAnnotations{
			Title:        "Search Documentation",
			ReadOnlyHint: true,
		},
	}

	// Thanos-specific tools.
	thanosStoresToolDef = &mcp.Tool{
		Name:        "list_stores",
		Description: "List all store API servers",
		Annotations: &mcp.ToolAnnotations{
			Title:        "List Stores",
			ReadOnlyHint: true,
		},
	}
)
