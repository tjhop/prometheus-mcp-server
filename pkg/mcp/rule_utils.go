package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// parseLabelsFromJSON parses a JSON string into a map of labels
func parseLabelsFromJSON(jsonStr string) (map[string]string, error) {
	var labels map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &labels); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}
	return labels, nil
}

// loadRuleFile loads a Prometheus rule file from disk
func loadRuleFile(filePath string) (*PrometheusRule, error) {
	var rule PrometheusRule
	
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// Create an empty rule structure
		return &PrometheusRule{Groups: []RuleGroup{}}, nil
	}
	
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	
	if err := yaml.Unmarshal(data, &rule); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from %s: %w", filePath, err)
	}
	
	return &rule, nil
}

// saveRuleFile saves a Prometheus rule file to disk
func saveRuleFile(filePath string, rule *PrometheusRule) error {
	data, err := yaml.Marshal(rule)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}
	
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	
	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}
	
	return nil
}

// findRuleGroup finds a rule group by name in the rule file
func findRuleGroup(rule *PrometheusRule, groupName string) (*RuleGroup, int) {
	for i, group := range rule.Groups {
		if group.Name == groupName {
			return &rule.Groups[i], i
		}
	}
	return nil, -1
}

// findRule finds a rule by name in a rule group
func findRule(group *RuleGroup, ruleName string) (*Rule, int) {
	for i, rule := range group.Rules {
		if rule.Record == ruleName || rule.Alert == ruleName {
			return &group.Rules[i], i
		}
	}
	return nil, -1
}

// addRuleToFile adds a new rule to a rule file
func addRuleToFile(filePath, groupName, groupInterval string, newRule Rule) error {
	rule, err := loadRuleFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to load rule file: %w", err)
	}
	
	// Find or create the rule group
	group, groupIdx := findRuleGroup(rule, groupName)
	if group == nil {
		// Create new group
		newGroup := RuleGroup{
			Name:     groupName,
			Interval: groupInterval,
			Rules:    []Rule{newRule},
		}
		rule.Groups = append(rule.Groups, newGroup)
	} else {
		// Check if rule already exists
		existingRule, _ := findRule(group, getRuleName(newRule))
		if existingRule != nil {
			return fmt.Errorf("rule '%s' already exists in group '%s'", getRuleName(newRule), groupName)
		}
		
		// Add rule to existing group
		rule.Groups[groupIdx].Rules = append(rule.Groups[groupIdx].Rules, newRule)
	}
	
	return saveRuleFile(filePath, rule)
}

// updateRuleInFile updates an existing rule in a rule file
func updateRuleInFile(filePath, groupName, ruleName, expr, forDuration string, labels, annotations map[string]string) error {
	rule, err := loadRuleFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to load rule file: %w", err)
	}
	
	// Find the rule group
	group, groupIdx := findRuleGroup(rule, groupName)
	if group == nil {
		return fmt.Errorf("rule group '%s' not found", groupName)
	}
	
	// Find the rule
	existingRule, ruleIdx := findRule(group, ruleName)
	if existingRule == nil {
		return fmt.Errorf("rule '%s' not found in group '%s'", ruleName, groupName)
	}
	
	// Update rule fields if provided
	if expr != "" {
		rule.Groups[groupIdx].Rules[ruleIdx].Expr = expr
	}
	if forDuration != "" {
		rule.Groups[groupIdx].Rules[ruleIdx].For = forDuration
	}
	if labels != nil {
		rule.Groups[groupIdx].Rules[ruleIdx].Labels = labels
	}
	if annotations != nil {
		rule.Groups[groupIdx].Rules[ruleIdx].Annotations = annotations
	}
	
	return saveRuleFile(filePath, rule)
}

// deleteRuleFromFile deletes a rule from a rule file
func deleteRuleFromFile(filePath, groupName, ruleName string) error {
	rule, err := loadRuleFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to load rule file: %w", err)
	}
	
	// Find the rule group
	group, groupIdx := findRuleGroup(rule, groupName)
	if group == nil {
		return fmt.Errorf("rule group '%s' not found", groupName)
	}
	
	// Find the rule
	_, ruleIdx := findRule(group, ruleName)
	if ruleIdx == -1 {
		return fmt.Errorf("rule '%s' not found in group '%s'", ruleName, groupName)
	}
	
	// Remove the rule from the group
	rule.Groups[groupIdx].Rules = append(
		rule.Groups[groupIdx].Rules[:ruleIdx],
		rule.Groups[groupIdx].Rules[ruleIdx+1:]...,
	)
	
	// Remove the group if it's empty
	if len(rule.Groups[groupIdx].Rules) == 0 {
		rule.Groups = append(rule.Groups[:groupIdx], rule.Groups[groupIdx+1:]...)
	}
	
	return saveRuleFile(filePath, rule)
}

// getRuleName extracts the rule name from a rule (either record or alert)
func getRuleName(rule Rule) string {
	if rule.Record != "" {
		return rule.Record
	}
	return rule.Alert
}

// validatePromQLExpression validates a PromQL expression using the Prometheus API
func validatePromQLExpression(ctx context.Context, expr string) (bool, error) {
	// For now, we'll implement a basic validation
	// In a full implementation, you'd want to use the Prometheus /api/v1/parse_query endpoint
	
	// Basic validation - check if expression is not empty and contains valid characters
	if strings.TrimSpace(expr) == "" {
		return false, nil
	}
	
	// You could enhance this by making an actual API call to Prometheus
	// For now, we'll assume it's valid if it's not empty
	return true, nil
}

// listRuleFiles lists all rule files in a directory matching a pattern
func listRuleFiles(directoryPath, pattern string) ([]string, error) {
	var files []string
	
	// Use filepath.Glob to find matching files
	searchPattern := filepath.Join(directoryPath, pattern)
	matches, err := filepath.Glob(searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to search for files: %w", err)
	}
	
	for _, match := range matches {
		// Get relative path from directory
		relPath, err := filepath.Rel(directoryPath, match)
		if err != nil {
			relPath = match
		}
		files = append(files, relPath)
	}
	
	return files, nil
}

// getRuleFileContent reads and returns the content of a rule file
func getRuleFileContent(filePath string) (string, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	
	return string(data), nil
}

// reloadPrometheusConfig triggers a Prometheus configuration reload
func reloadPrometheusConfig(ctx context.Context, prometheusURL string) error {
	// If no URL provided, use the default from the global client
	if prometheusURL == "" {
		// For now, we'll assume the user provides the URL
		// In a full implementation, you'd get this from the global configuration
		return fmt.Errorf("prometheus_url must be provided")
	}
	
	// Ensure URL ends with /-/reload
	if !strings.HasSuffix(prometheusURL, "/-/reload") {
		if !strings.HasSuffix(prometheusURL, "/") {
			prometheusURL += "/"
		}
		prometheusURL += "-/reload"
	}
	
	// Make HTTP POST request to reload endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", prometheusURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reload request failed with status %d", resp.StatusCode)
	}
	
	return nil
}
