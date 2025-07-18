package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// Simple integration test to verify rule tool handlers work
func TestRuleToolHandlers(t *testing.T) {
	// Test that the handlers can be called without panicking
	// More comprehensive tests are in rule_utils_test.go
	
	// Create a simple request
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: map[string]interface{}{
				"expr": "up{job=\"test\"}",
			},
		},
	}

	// Test validate rule handler
	_, err := validateRuleToolHandler(context.Background(), req)
	if err != nil {
		t.Errorf("validateRuleToolHandler failed: %v", err)
	}
}

