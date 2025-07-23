# Prometheus Rule Management Tools

This document describes the new rule management tools that have been added to the prometheus-mcp-server.

## Overview

The prometheus-mcp-server now includes comprehensive rule management capabilities that allow creating, updating, deleting, and managing Prometheus alerting and recording rules through file operations and configuration reloads.

## New Tools Added

### 1. Rule Creation Tools

#### `create_recording_rule`
Creates a new recording rule in a Prometheus rule file.

**Parameters:**
- `rule_file_path` (required): Path to the rule file (YAML)
- `group_name` (required): Name of the rule group
- `record_name` (required): Name of the recording rule (metric name)
- `expr` (required): PromQL expression
- `group_interval` (optional): Evaluation interval for new groups
- `labels` (optional): JSON object string of labels

**Example:**
```json
{
  "rule_file_path": "/etc/prometheus/rules/app.yml",
  "group_name": "application_metrics",
  "record_name": "app:request_rate_5m",
  "expr": "rate(http_requests_total[5m])",
  "labels": "{\"team\":\"platform\"}"
}
```

#### `create_alerting_rule`
Creates a new alerting rule in a Prometheus rule file.

**Parameters:**
- `rule_file_path` (required): Path to the rule file (YAML)
- `group_name` (required): Name of the rule group
- `alert_name` (required): Name of the alerting rule
- `expr` (required): PromQL expression
- `for_duration` (optional): Duration before firing
- `group_interval` (optional): Evaluation interval for new groups
- `labels` (optional): JSON object string of labels
- `annotations` (optional): JSON object string of annotations

**Example:**
```json
{
  "rule_file_path": "/etc/prometheus/rules/alerts.yml",
  "group_name": "application_alerts",
  "alert_name": "HighErrorRate",
  "expr": "rate(http_requests_total{status=~\"5..\"}[5m]) > 0.05",
  "for_duration": "5m",
  "labels": "{\"severity\":\"critical\"}",
  "annotations": "{\"summary\":\"High error rate detected\"}"
}
```

### 2. Rule Management Tools

#### `update_rule`
Updates an existing rule in a Prometheus rule file.

**Parameters:**
- `rule_file_path` (required): Path to the rule file
- `group_name` (required): Name of the rule group
- `rule_name` (required): Name of the rule to update
- `expr` (optional): New PromQL expression
- `for_duration` (optional): New duration for alerting rules
- `labels` (optional): New labels as JSON object string
- `annotations` (optional): New annotations as JSON object string

#### `delete_rule`
Deletes a rule from a Prometheus rule file.

**Parameters:**
- `rule_file_path` (required): Path to the rule file
- `group_name` (required): Name of the rule group
- `rule_name` (required): Name of the rule to delete

### 3. Validation and Utility Tools

#### `validate_rule`
Validates a PromQL expression for syntax correctness.

**Parameters:**
- `expr` (required): PromQL expression to validate

#### `list_rule_files`
Lists all rule files in a directory.

**Parameters:**
- `directory_path` (required): Directory to search
- `pattern` (optional): File pattern (defaults to "*.yml")

#### `get_rule_file_content`
Gets the content of a rule file.

**Parameters:**
- `rule_file_path` (required): Path to the rule file

### 4. Configuration Management

#### `reload_config`
Triggers a Prometheus configuration reload.

**Parameters:**
- `prometheus_url` (optional): Prometheus server URL

## File Structure

The implementation consists of three main files:

### `pkg/mcp/rule_tools.go`
- Defines the tool specifications and handlers
- Contains the main tool logic and parameter validation
- Integrates with the MCP framework

### `pkg/mcp/rule_utils.go`
- Contains utility functions for rule file operations
- Handles YAML parsing and file I/O
- Implements rule finding, adding, updating, and deleting logic

### `pkg/mcp/rule_utils_test.go`
- Comprehensive test suite for all utility functions
- Tests file operations, rule manipulation, and validation
- Uses temporary files for safe testing

## Features

### Rule File Management
- **Automatic file creation**: Creates rule files if they don't exist
- **Group management**: Creates new groups or adds to existing ones
- **Duplicate detection**: Prevents creation of duplicate rules
- **Safe deletion**: Removes empty groups after deleting last rule

### Data Validation
- **JSON parsing**: Validates labels and annotations as JSON
- **Parameter validation**: Ensures required fields are provided
- **File path validation**: Checks file accessibility

### Error Handling
- **Comprehensive error messages**: Clear feedback on failures
- **Safe operations**: Validates before making changes
- **Rollback safety**: Operations are atomic where possible

## Usage Examples

### Creating a Recording Rule
```bash
# Create a recording rule for calculating request rates
create_recording_rule(
  rule_file_path="/etc/prometheus/rules/app.yml",
  group_name="application_metrics",
  record_name="app:request_rate_5m",
  expr="rate(http_requests_total[5m])",
  labels='{"team":"platform","component":"api"}'
)
```

### Creating an Alerting Rule
```bash
# Create an alert for high error rates
create_alerting_rule(
  rule_file_path="/etc/prometheus/rules/alerts.yml",
  group_name="application_alerts",
  alert_name="HighErrorRate",
  expr="rate(http_requests_total{status=~\"5..\"}[5m]) > 0.05",
  for_duration="5m",
  labels='{"severity":"critical","team":"platform"}',
  annotations='{"summary":"High error rate detected","description":"Error rate is above 5% for 5 minutes"}'
)
```

### Updating a Rule
```bash
# Update the threshold for an existing alert
update_rule(
  rule_file_path="/etc/prometheus/rules/alerts.yml",
  group_name="application_alerts", 
  rule_name="HighErrorRate",
  expr="rate(http_requests_total{status=~\"5..\"}[5m]) > 0.02"
)
```

### Reloading Configuration
```bash
# Trigger Prometheus to reload its configuration
reload_config(prometheus_url="http://localhost:9090")
```

## Testing

The implementation includes comprehensive tests covering:
- JSON label parsing
- File I/O operations
- Rule manipulation (add/update/delete)
- Group management
- Error conditions
- Edge cases

Run tests with:
```bash
go test ./pkg/mcp -v
```

## Installation Requirements

- Go 1.24+ 
- YAML support: `gopkg.in/yaml.v2`
- File system write access for rule files
- Network access to Prometheus server for configuration reloads

## Security Considerations

- **File permissions**: Ensure proper permissions on rule files
- **Path validation**: Validate file paths to prevent directory traversal
- **Network security**: Secure communication with Prometheus server
- **Access control**: Limit who can use these destructive operations

## Future Enhancements

- **Rule validation**: Integration with Prometheus rule validation API
- **Backup/restore**: Automatic backups before making changes  
- **Batch operations**: Support for multiple rule operations
- **Rule templates**: Predefined rule templates for common scenarios
- **Rule dependencies**: Validation of rule dependencies
