package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// Rule structures based on Prometheus rule format
type PrometheusRule struct {
	Groups []RuleGroup `yaml:"groups"`
}

type RuleGroup struct {
	Name     string `yaml:"name"`
	Interval string `yaml:"interval,omitempty"`
	Rules    []Rule `yaml:"rules"`
}

type Rule struct {
	// Common fields
	Record string            `yaml:"record,omitempty"`
	Alert  string            `yaml:"alert,omitempty"`
	Expr   string            `yaml:"expr"`
	For    string            `yaml:"for,omitempty"`
	Labels map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

var (
	// Rule management tools
	createRecordingRuleTool = mcp.NewTool("create_recording_rule",
		mcp.WithDescription("Create a new recording rule in a Prometheus rule file"),
		mcp.WithString("rule_file_path",
			mcp.Required(),
			mcp.Description("Path to the rule file (YAML) where the rule should be created"),
		),
		mcp.WithString("group_name",
			mcp.Required(),
			mcp.Description("Name of the rule group to add the rule to (will be created if it doesn't exist)"),
		),
		mcp.WithString("record_name",
			mcp.Required(),
			mcp.Description("Name of the recording rule (metric name to record)"),
		),
		mcp.WithString("expr",
			mcp.Required(),
			mcp.Description("PromQL expression for the recording rule"),
		),
		mcp.WithString("group_interval",
			mcp.Description("[Optional] Evaluation interval for the rule group (e.g., '30s', '1m'). Only used if creating a new group."),
		),
		mcp.WithString("labels",
			mcp.Description("[Optional] JSON object string of labels to add to the recorded metric (e.g., '{\"severity\":\"warning\"}')"),
		),
	)

	createAlertingRuleTool = mcp.NewTool("create_alerting_rule",
		mcp.WithDescription("Create a new alerting rule in a Prometheus rule file"),
		mcp.WithString("rule_file_path",
			mcp.Required(),
			mcp.Description("Path to the rule file (YAML) where the rule should be created"),
		),
		mcp.WithString("group_name",
			mcp.Required(),
			mcp.Description("Name of the rule group to add the rule to (will be created if it doesn't exist)"),
		),
		mcp.WithString("alert_name",
			mcp.Required(),
			mcp.Description("Name of the alerting rule"),
		),
		mcp.WithString("expr",
			mcp.Required(),
			mcp.Description("PromQL expression for the alerting rule"),
		),
		mcp.WithString("for_duration",
			mcp.Description("[Optional] Duration for which the condition must be true before firing (e.g., '5m', '1h')"),
		),
		mcp.WithString("group_interval",
			mcp.Description("[Optional] Evaluation interval for the rule group (e.g., '30s', '1m'). Only used if creating a new group."),
		),
		mcp.WithString("labels",
			mcp.Description("[Optional] JSON object string of labels to add to the alert (e.g., '{\"severity\":\"warning\"}')"),
		),
		mcp.WithString("annotations",
			mcp.Description("[Optional] JSON object string of annotations for the alert (e.g., '{\"summary\":\"High CPU usage\"}')"),
		),
	)

	updateRuleTool = mcp.NewTool("update_rule",
		mcp.WithDescription("Update an existing rule in a Prometheus rule file"),
		mcp.WithString("rule_file_path",
			mcp.Required(),
			mcp.Description("Path to the rule file (YAML) containing the rule to update"),
		),
		mcp.WithString("group_name",
			mcp.Required(),
			mcp.Description("Name of the rule group containing the rule"),
		),
		mcp.WithString("rule_name",
			mcp.Required(),
			mcp.Description("Name of the rule to update (record name for recording rules, alert name for alerting rules)"),
		),
		mcp.WithString("expr",
			mcp.Description("[Optional] New PromQL expression for the rule"),
		),
		mcp.WithString("for_duration",
			mcp.Description("[Optional] New duration for alerting rules (e.g., '5m', '1h')"),
		),
		mcp.WithString("labels",
			mcp.Description("[Optional] New labels as JSON object string (e.g., '{\"severity\":\"critical\"}')"),
		),
		mcp.WithString("annotations",
			mcp.Description("[Optional] New annotations as JSON object string (e.g., '{\"summary\":\"Updated alert\"}')"),
		),
	)

	deleteRuleTool = mcp.NewTool("delete_rule",
		mcp.WithDescription("Delete a rule from a Prometheus rule file"),
		mcp.WithString("rule_file_path",
			mcp.Required(),
			mcp.Description("Path to the rule file (YAML) containing the rule to delete"),
		),
		mcp.WithString("group_name",
			mcp.Required(),
			mcp.Description("Name of the rule group containing the rule"),
		),
		mcp.WithString("rule_name",
			mcp.Required(),
			mcp.Description("Name of the rule to delete (record name for recording rules, alert name for alerting rules)"),
		),
	)

	validateRuleTool = mcp.NewTool("validate_rule",
		mcp.WithDescription("Validate a PromQL expression for syntax correctness"),
		mcp.WithString("expr",
			mcp.Required(),
			mcp.Description("PromQL expression to validate"),
		),
	)

	listRuleFilesTool = mcp.NewTool("list_rule_files",
		mcp.WithDescription("List all rule files in a directory"),
		mcp.WithString("directory_path",
			mcp.Required(),
			mcp.Description("Directory path to search for rule files"),
		),
		mcp.WithString("pattern",
			mcp.Description("[Optional] File pattern to match (e.g., '*.yml', '*.yaml'). Defaults to '*.yml'"),
		),
	)

	getRuleFileContentTool = mcp.NewTool("get_rule_file_content",
		mcp.WithDescription("Get the content of a rule file"),
		mcp.WithString("rule_file_path",
			mcp.Required(),
			mcp.Description("Path to the rule file to read"),
		),
	)

	reloadConfigTool = mcp.NewTool("reload_config",
		mcp.WithDescription("Trigger Prometheus configuration reload"),
		mcp.WithString("prometheus_url",
			mcp.Description("[Optional] Prometheus server URL. Defaults to configured URL."),
		),
	)
)

// Rule management tool handlers
func createRecordingRuleToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ruleFilePath, err := request.RequireString("rule_file_path")
	if err != nil {
		return mcp.NewToolResultError("rule_file_path must be a string"), nil
	}

	groupName, err := request.RequireString("group_name")
	if err != nil {
		return mcp.NewToolResultError("group_name must be a string"), nil
	}

	recordName, err := request.RequireString("record_name")
	if err != nil {
		return mcp.NewToolResultError("record_name must be a string"), nil
	}

	expr, err := request.RequireString("expr")
	if err != nil {
		return mcp.NewToolResultError("expr must be a string"), nil
	}

	groupInterval := request.GetString("group_interval", "")
	labelsStr := request.GetString("labels", "")

	// Parse labels if provided
	var labels map[string]string
	if labelsStr != "" {
		labels, err = parseLabelsFromJSON(labelsStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid labels JSON: %v", err)), nil
		}
	}

	// Create the recording rule
	rule := Rule{
		Record: recordName,
		Expr:   expr,
		Labels: labels,
	}

	err = addRuleToFile(ruleFilePath, groupName, groupInterval, rule)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create recording rule: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully created recording rule '%s' in group '%s' in file '%s'", recordName, groupName, ruleFilePath)), nil
}

func createAlertingRuleToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ruleFilePath, err := request.RequireString("rule_file_path")
	if err != nil {
		return mcp.NewToolResultError("rule_file_path must be a string"), nil
	}

	groupName, err := request.RequireString("group_name")
	if err != nil {
		return mcp.NewToolResultError("group_name must be a string"), nil
	}

	alertName, err := request.RequireString("alert_name")
	if err != nil {
		return mcp.NewToolResultError("alert_name must be a string"), nil
	}

	expr, err := request.RequireString("expr")
	if err != nil {
		return mcp.NewToolResultError("expr must be a string"), nil
	}

	forDuration := request.GetString("for_duration", "")
	groupInterval := request.GetString("group_interval", "")
	labelsStr := request.GetString("labels", "")
	annotationsStr := request.GetString("annotations", "")

	// Parse labels and annotations if provided
	var labels, annotations map[string]string
	if labelsStr != "" {
		labels, err = parseLabelsFromJSON(labelsStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid labels JSON: %v", err)), nil
		}
	}
	if annotationsStr != "" {
		annotations, err = parseLabelsFromJSON(annotationsStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid annotations JSON: %v", err)), nil
		}
	}

	// Create the alerting rule
	rule := Rule{
		Alert:       alertName,
		Expr:        expr,
		For:         forDuration,
		Labels:      labels,
		Annotations: annotations,
	}

	err = addRuleToFile(ruleFilePath, groupName, groupInterval, rule)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create alerting rule: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully created alerting rule '%s' in group '%s' in file '%s'", alertName, groupName, ruleFilePath)), nil
}

func updateRuleToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ruleFilePath, err := request.RequireString("rule_file_path")
	if err != nil {
		return mcp.NewToolResultError("rule_file_path must be a string"), nil
	}

	groupName, err := request.RequireString("group_name")
	if err != nil {
		return mcp.NewToolResultError("group_name must be a string"), nil
	}

	ruleName, err := request.RequireString("rule_name")
	if err != nil {
		return mcp.NewToolResultError("rule_name must be a string"), nil
	}

	expr := request.GetString("expr", "")
	forDuration := request.GetString("for_duration", "")
	labelsStr := request.GetString("labels", "")
	annotationsStr := request.GetString("annotations", "")

	// Parse labels and annotations if provided
	var labels, annotations map[string]string
	if labelsStr != "" {
		labels, err = parseLabelsFromJSON(labelsStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid labels JSON: %v", err)), nil
		}
	}
	if annotationsStr != "" {
		annotations, err = parseLabelsFromJSON(annotationsStr)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid annotations JSON: %v", err)), nil
		}
	}

	err = updateRuleInFile(ruleFilePath, groupName, ruleName, expr, forDuration, labels, annotations)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to update rule: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully updated rule '%s' in group '%s' in file '%s'", ruleName, groupName, ruleFilePath)), nil
}

func deleteRuleToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ruleFilePath, err := request.RequireString("rule_file_path")
	if err != nil {
		return mcp.NewToolResultError("rule_file_path must be a string"), nil
	}

	groupName, err := request.RequireString("group_name")
	if err != nil {
		return mcp.NewToolResultError("group_name must be a string"), nil
	}

	ruleName, err := request.RequireString("rule_name")
	if err != nil {
		return mcp.NewToolResultError("rule_name must be a string"), nil
	}

	err = deleteRuleFromFile(ruleFilePath, groupName, ruleName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to delete rule: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully deleted rule '%s' from group '%s' in file '%s'", ruleName, groupName, ruleFilePath)), nil
}

func validateRuleToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	expr, err := request.RequireString("expr")
	if err != nil {
		return mcp.NewToolResultError("expr must be a string"), nil
	}

	// Use the existing parse query API call if available
	isValid, err := validatePromQLExpression(ctx, expr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to validate expression: %v", err)), nil
	}

	if isValid {
		return mcp.NewToolResultText(fmt.Sprintf("PromQL expression is valid: %s", expr)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("PromQL expression is invalid: %s", expr)), nil
}

func listRuleFilesToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	directoryPath, err := request.RequireString("directory_path")
	if err != nil {
		return mcp.NewToolResultError("directory_path must be a string"), nil
	}

	pattern := request.GetString("pattern", "*.yml")

	files, err := listRuleFiles(directoryPath, pattern)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list rule files: %v", err)), nil
	}

	if len(files) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No rule files found in directory '%s' with pattern '%s'", directoryPath, pattern)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d rule files in '%s':\n%s", len(files), directoryPath, strings.Join(files, "\n"))), nil
}

func getRuleFileContentToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ruleFilePath, err := request.RequireString("rule_file_path")
	if err != nil {
		return mcp.NewToolResultError("rule_file_path must be a string"), nil
	}

	content, err := getRuleFileContent(ruleFilePath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to read rule file: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Content of rule file '%s':\n\n%s", ruleFilePath, content)), nil
}

func reloadConfigToolHandler(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	prometheusURL := request.GetString("prometheus_url", "")

	err := reloadPrometheusConfig(ctx, prometheusURL)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to reload Prometheus configuration: %v", err)), nil
	}

	return mcp.NewToolResultText("Successfully triggered Prometheus configuration reload"), nil
}
