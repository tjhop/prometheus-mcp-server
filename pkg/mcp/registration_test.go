package mcp

import (
	"log/slog"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"
)

// getToolNames returns the names of all tools in a toolset map.
// The returned slice is sorted for consistent comparison in tests.
// This is a test helper function.
func getToolNames(toolset map[string]toolRegistration) []string {
	names := make([]string, 0, len(toolset))
	for name := range toolset {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func TestGetToolset(t *testing.T) {
	t.Run("all tools loads entire prometheus toolset", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools: []string{"all"},
			Logger:       slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Verify the returned toolset exactly matches the prometheus toolset.
		expectedNames := getToolNames(prometheusToolset)
		require.ElementsMatch(t, expectedNames, names, "toolset should contain exactly the prometheus toolset")
	})

	t.Run("core tools loads only core subset", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools: []string{"core"},
			Logger:       slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Should contain exactly the core tools.
		for _, coreTool := range CoreTools {
			require.Contains(t, names, coreTool, "expected core tool %q to be present", coreTool)
		}

		// Should NOT contain non-core tools.
		require.NotContains(t, names, "config")
		require.NotContains(t, names, "quit")

		require.Len(t, toolset, len(CoreTools))
	})

	t.Run("custom tool list includes core plus specified tools", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools: []string{"alertmanagers", "config"},
			Logger:       slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Should contain core tools.
		for _, coreTool := range CoreTools {
			require.Contains(t, names, coreTool, "expected core tool %q to be present", coreTool)
		}

		// Should contain the specified additional tools.
		require.Contains(t, names, "alertmanagers")
		require.Contains(t, names, "config")

		// Should NOT contain other non-core tools not in the list.
		require.NotContains(t, names, "reload")
		require.NotContains(t, names, "quit")
	})

	t.Run("invalid tool names are ignored with warning", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools: []string{"nonexistent_tool", "alertmanagers"},
			Logger:       slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Should still have core tools and valid specified tools.
		require.Contains(t, names, "alertmanagers")
		for _, coreTool := range CoreTools {
			require.Contains(t, names, coreTool)
		}

		// Invalid tool should not be present.
		require.NotContains(t, names, "nonexistent_tool")
	})

	t.Run("thanos backend overrides toolset with thanos-specific tools", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools:      []string{"all"}, // This would normally load prometheus
			prometheusBackend: "thanos",
			Logger:            slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Thanos-specific tool should be present.
		require.Contains(t, names, "list_stores")

		// Prometheus-only tools should NOT be present.
		require.NotContains(t, names, "alertmanagers")
		require.NotContains(t, names, "config")
		require.NotContains(t, names, "wal_replay_status")
		require.NotContains(t, names, "reload")
		require.NotContains(t, names, "quit")

		// Shared tools should still be present.
		require.Contains(t, names, "query")
		require.Contains(t, names, "range_query")
		require.Contains(t, names, "docs_search")

		// Should have exactly the thanos toolset size.
		require.Len(t, toolset, len(thanosToolset))
	})

	t.Run("prometheus backend explicitly loads full prometheus toolset", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools:      []string{"core"}, // Would normally just load core
			prometheusBackend: "prometheus",
			Logger:            slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Backend override should give us all prometheus tools.
		require.Contains(t, names, "alertmanagers")
		require.Contains(t, names, "config")
		require.Contains(t, names, "reload")
		require.Len(t, toolset, len(prometheusToolset))
	})

	t.Run("unknown backend keeps existing toolset with warning", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools:      []string{"alertmanagers"},
			prometheusBackend: "super-awesome-tsdb", // Unknown backend
			Logger:            slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Should keep the custom toolset (core + alertmanagers).
		require.Contains(t, names, "alertmanagers")
		for _, coreTool := range CoreTools {
			require.Contains(t, names, coreTool)
		}

		// Should NOT have thanos-specific tools.
		require.NotContains(t, names, "list_stores")
	})

	t.Run("empty enabled tools loads only core", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools: []string{},
			Logger:       slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Should contain exactly core tools.
		for _, coreTool := range CoreTools {
			require.Contains(t, names, coreTool)
		}
		require.Len(t, toolset, len(CoreTools))
	})

	t.Run("duplicate tools in enabled list are deduplicated", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools: []string{"query", "query", "alertmanagers", "alertmanagers"},
			Logger:       slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		// Count occurrences of each tool.
		counts := make(map[string]int)
		for _, name := range names {
			counts[name]++
		}

		// No duplicates should exist.
		for name, count := range counts {
			require.Equal(t, 1, count, "tool %s should appear exactly once", name)
		}
	})

	t.Run("case insensitive backend matching", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools:      []string{"core"},
			prometheusBackend: "THANOS",
			Logger:            slog.Default(),
		}

		toolset := getToolset(cfg)
		names := getToolNames(toolset)

		require.Contains(t, names, "list_stores")
		require.NotContains(t, names, "alertmanagers")
	})

	t.Run("nil logger uses default", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools: []string{"core"},
			Logger:       nil,
		}

		require.NotPanics(t, func() {
			toolset := getToolset(cfg)
			require.NotNil(t, toolset)
			require.Len(t, toolset, len(CoreTools))
		})
	})

	t.Run("mixed case backend prometheus", func(t *testing.T) {
		cfg := toolsetConfig{
			enabledTools:      []string{"core"},
			prometheusBackend: "Prometheus", // Mixed case
			Logger:            slog.Default(),
		}

		toolset := getToolset(cfg)

		require.Len(t, toolset, len(prometheusToolset))
	})
}

// TestToolsetContents verifies the actual contents of the toolsets.
func TestToolsetContents(t *testing.T) {
	t.Run("thanosToolset excludes prometheus-only tools", func(t *testing.T) {
		prometheusOnly := []string{
			"alertmanagers",
			"config",
			"wal_replay_status",
			"reload",
			"quit",
		}

		for _, tool := range prometheusOnly {
			_, exists := thanosToolset[tool]
			require.False(t, exists, "thanos toolset should not contain %s", tool)
		}
	})

	t.Run("thanosToolset includes thanos-specific tools", func(t *testing.T) {
		_, exists := thanosToolset["list_stores"]
		require.True(t, exists, "thanos toolset should contain list_stores")
	})

	t.Run("all core tools exist in prometheusToolset", func(t *testing.T) {
		for _, coreTool := range CoreTools {
			_, exists := prometheusToolset[coreTool]
			require.True(t, exists, "core tool %s should exist in prometheusToolset", coreTool)
		}
	})

	t.Run("all TSDB admin tools exist in prometheusToolset", func(t *testing.T) {
		for _, adminTool := range PrometheusTsdbAdminTools {
			_, exists := prometheusToolset[adminTool]
			require.True(t, exists, "TSDB admin tool %s should exist in prometheusToolset", adminTool)
		}
	})
}
