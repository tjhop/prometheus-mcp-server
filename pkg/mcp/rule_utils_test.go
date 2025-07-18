package mcp

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLabelsFromJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    map[string]string
		expectError bool
	}{
		{
			name:     "Valid JSON",
			input:    `{"severity":"critical","team":"platform"}`,
			expected: map[string]string{"severity": "critical", "team": "platform"},
		},
		{
			name:        "Invalid JSON",
			input:       `{"severity":"critical"`,
			expectError: true,
		},
		{
			name:     "Empty JSON",
			input:    `{}`,
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseLabelsFromJSON(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d labels, got %d", len(tt.expected), len(result))
				return
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("Expected label %s=%s, got %s", k, v, result[k])
				}
			}
		})
	}
}

func TestLoadAndSaveRuleFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := ioutil.TempDir("", "rule_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test_rules.yml")

	// Test loading non-existent file
	rule, err := loadRuleFile(testFile)
	if err != nil {
		t.Errorf("Expected no error when loading non-existent file, got: %v", err)
	}
	if len(rule.Groups) != 0 {
		t.Errorf("Expected empty rule groups, got %d", len(rule.Groups))
	}

	// Create a rule and save it
	rule.Groups = []RuleGroup{
		{
			Name:     "test_group",
			Interval: "30s",
			Rules: []Rule{
				{
					Record: "test_metric",
					Expr:   "up{job=\"test\"}",
					Labels: map[string]string{"team": "platform"},
				},
			},
		},
	}

	err = saveRuleFile(testFile, rule)
	if err != nil {
		t.Errorf("Failed to save rule file: %v", err)
	}

	// Load the saved file and verify
	loadedRule, err := loadRuleFile(testFile)
	if err != nil {
		t.Errorf("Failed to load saved rule file: %v", err)
	}

	if len(loadedRule.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(loadedRule.Groups))
	}

	group := loadedRule.Groups[0]
	if group.Name != "test_group" {
		t.Errorf("Expected group name 'test_group', got '%s'", group.Name)
	}

	if len(group.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(group.Rules))
	}

	rule_item := group.Rules[0]
	if rule_item.Record != "test_metric" {
		t.Errorf("Expected record 'test_metric', got '%s'", rule_item.Record)
	}
}

func TestFindRuleGroup(t *testing.T) {
	rule := &PrometheusRule{
		Groups: []RuleGroup{
			{Name: "group1", Rules: []Rule{}},
			{Name: "group2", Rules: []Rule{}},
		},
	}

	// Test finding existing group
	group, idx := findRuleGroup(rule, "group1")
	if group == nil {
		t.Errorf("Expected to find group1, got nil")
	}
	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}

	// Test finding non-existent group
	group, idx = findRuleGroup(rule, "nonexistent")
	if group != nil {
		t.Errorf("Expected nil for non-existent group, got %v", group)
	}
	if idx != -1 {
		t.Errorf("Expected index -1, got %d", idx)
	}
}

func TestFindRule(t *testing.T) {
	group := &RuleGroup{
		Rules: []Rule{
			{Record: "metric1", Expr: "up"},
			{Alert: "alert1", Expr: "down"},
		},
	}

	// Test finding recording rule
	rule, idx := findRule(group, "metric1")
	if rule == nil {
		t.Errorf("Expected to find metric1, got nil")
	}
	if idx != 0 {
		t.Errorf("Expected index 0, got %d", idx)
	}

	// Test finding alerting rule
	rule, idx = findRule(group, "alert1")
	if rule == nil {
		t.Errorf("Expected to find alert1, got nil")
	}
	if idx != 1 {
		t.Errorf("Expected index 1, got %d", idx)
	}

	// Test finding non-existent rule
	rule, idx = findRule(group, "nonexistent")
	if rule != nil {
		t.Errorf("Expected nil for non-existent rule, got %v", rule)
	}
	if idx != -1 {
		t.Errorf("Expected index -1, got %d", idx)
	}
}

func TestAddRuleToFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := ioutil.TempDir("", "rule_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test_rules.yml")

	// Test adding rule to new file
	newRule := Rule{
		Record: "test_metric",
		Expr:   "up{job=\"test\"}",
		Labels: map[string]string{"team": "platform"},
	}

	err = addRuleToFile(testFile, "test_group", "30s", newRule)
	if err != nil {
		t.Errorf("Failed to add rule to new file: %v", err)
	}

	// Verify the rule was added
	rule, err := loadRuleFile(testFile)
	if err != nil {
		t.Errorf("Failed to load rule file after adding: %v", err)
	}

	if len(rule.Groups) != 1 {
		t.Errorf("Expected 1 group, got %d", len(rule.Groups))
	}

	if len(rule.Groups[0].Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(rule.Groups[0].Rules))
	}

	// Test adding another rule to existing group
	newRule2 := Rule{
		Alert: "test_alert",
		Expr:  "down{job=\"test\"}",
		For:   "5m",
	}

	err = addRuleToFile(testFile, "test_group", "", newRule2)
	if err != nil {
		t.Errorf("Failed to add rule to existing group: %v", err)
	}

	// Verify both rules exist
	rule, err = loadRuleFile(testFile)
	if err != nil {
		t.Errorf("Failed to load rule file after adding second rule: %v", err)
	}

	if len(rule.Groups[0].Rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(rule.Groups[0].Rules))
	}

	// Test adding duplicate rule (should fail)
	err = addRuleToFile(testFile, "test_group", "", newRule)
	if err == nil {
		t.Errorf("Expected error when adding duplicate rule, got nil")
	}
}

func TestDeleteRuleFromFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := ioutil.TempDir("", "rule_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test_rules.yml")

	// Create a rule file with multiple rules
	rule := &PrometheusRule{
		Groups: []RuleGroup{
			{
				Name: "test_group",
				Rules: []Rule{
					{Record: "metric1", Expr: "up"},
					{Alert: "alert1", Expr: "down"},
				},
			},
		},
	}

	err = saveRuleFile(testFile, rule)
	if err != nil {
		t.Errorf("Failed to save initial rule file: %v", err)
	}

	// Delete one rule
	err = deleteRuleFromFile(testFile, "test_group", "metric1")
	if err != nil {
		t.Errorf("Failed to delete rule: %v", err)
	}

	// Verify rule was deleted
	loadedRule, err := loadRuleFile(testFile)
	if err != nil {
		t.Errorf("Failed to load rule file after deletion: %v", err)
	}

	if len(loadedRule.Groups[0].Rules) != 1 {
		t.Errorf("Expected 1 rule after deletion, got %d", len(loadedRule.Groups[0].Rules))
	}

	if loadedRule.Groups[0].Rules[0].Alert != "alert1" {
		t.Errorf("Expected remaining rule to be 'alert1', got '%s'", loadedRule.Groups[0].Rules[0].Alert)
	}

	// Delete last rule (should remove group)
	err = deleteRuleFromFile(testFile, "test_group", "alert1")
	if err != nil {
		t.Errorf("Failed to delete last rule: %v", err)
	}

	// Verify group was deleted
	loadedRule, err = loadRuleFile(testFile)
	if err != nil {
		t.Errorf("Failed to load rule file after deleting last rule: %v", err)
	}

	if len(loadedRule.Groups) != 0 {
		t.Errorf("Expected 0 groups after deleting last rule, got %d", len(loadedRule.Groups))
	}
}

func TestGetRuleName(t *testing.T) {
	// Test recording rule
	recordingRule := Rule{Record: "test_metric", Expr: "up"}
	name := getRuleName(recordingRule)
	if name != "test_metric" {
		t.Errorf("Expected 'test_metric', got '%s'", name)
	}

	// Test alerting rule
	alertingRule := Rule{Alert: "test_alert", Expr: "down"}
	name = getRuleName(alertingRule)
	if name != "test_alert" {
		t.Errorf("Expected 'test_alert', got '%s'", name)
	}
}

func TestValidatePromQLExpression(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{
			name:     "Valid expression",
			expr:     "up{job=\"test\"}",
			expected: true,
		},
		{
			name:     "Empty expression",
			expr:     "",
			expected: false,
		},
		{
			name:     "Whitespace only",
			expr:     "   ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validatePromQLExpression(context.Background(), tt.expr)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestListRuleFiles(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := ioutil.TempDir("", "rule_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFiles := []string{"rules1.yml", "rules2.yaml", "config.txt"}
	for _, filename := range testFiles {
		err := ioutil.WriteFile(filepath.Join(tempDir, filename), []byte("test"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Test listing .yml files
	files, err := listRuleFiles(tempDir, "*.yml")
	if err != nil {
		t.Errorf("Failed to list rule files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 .yml file, got %d", len(files))
	}

	if files[0] != "rules1.yml" {
		t.Errorf("Expected 'rules1.yml', got '%s'", files[0])
	}

	// Test listing all yaml files
	files, err = listRuleFiles(tempDir, "*.y*")
	if err != nil {
		t.Errorf("Failed to list rule files: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 yaml files, got %d", len(files))
	}
}

func TestGetRuleFileContent(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := ioutil.TempDir("", "rule_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test_rules.yml")
	testContent := `groups:
  - name: test_group
    rules:
      - record: test_metric
        expr: up{job="test"}
`

	err = ioutil.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test reading file content
	content, err := getRuleFileContent(testFile)
	if err != nil {
		t.Errorf("Failed to read rule file content: %v", err)
	}

	if content != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, content)
	}

	// Test reading non-existent file
	_, err = getRuleFileContent(filepath.Join(tempDir, "nonexistent.yml"))
	if err == nil {
		t.Errorf("Expected error when reading non-existent file, got nil")
	}
}
