package mcp

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHandleDiscoverInsights_InputValidation(t *testing.T) {
	server := &MCPServer{}

	tests := []struct {
		name           string
		input          DiscoverInsightsParams
		expectError    bool
		expectInsights bool
	}{
		{
			name: "empty_profile",
			input: DiscoverInsightsParams{
				ProfileName: "",
				TableName:   "test",
			},
			expectError:    true,
			expectInsights: false,
		},
		{
			name: "empty_table",
			input: DiscoverInsightsParams{
				ProfileName: "test-profile",
				TableName:   "",
			},
			expectError:    true,
			expectInsights: false,
		},
		{
			name: "valid_params_but_no_profile",
			input: DiscoverInsightsParams{
				ProfileName: "nonexistent-profile",
				TableName:   "test",
			},
			expectError:    false, // Will fail at connection step
			expectInsights: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, err := server.handleDiscoverInsights(context.Background(), &mcp.CallToolRequest{}, tt.input)
			
			if err != nil {
				// Some errors are expected
				t.Logf("Got error (may be expected): %v", err)
			}
			
			if result != nil {
				// Check if result has error content (errorResult returns JSON error)
				hasErrorContent := len(result.Content) > 0
				if tt.expectError && !hasErrorContent {
					t.Errorf("Expected error content, got none")
				}
				if !tt.expectError && len(result.Content) > 0 {
					t.Logf("Got result with %d content items", len(result.Content))
				}
			} else if tt.expectError {
				t.Errorf("Expected error result but got nil")
			}
		})
	}
}

func TestHandleDiscoverInsights_WithAllParams(t *testing.T) {
	server := &MCPServer{}

	input := DiscoverInsightsParams{
		ProfileName:  "test-profile",
		TableName:    "users",
		Columns:      []string{"id", "name", "email"},
		InsightTypes: []InsightType{InsightTypeKPI, InsightTypeTrend},
		MaxResults:   10,
	}

	result, _, err := server.handleDiscoverInsights(context.Background(), &mcp.CallToolRequest{}, input)
	
	// This will fail because there's no actual profile configured
	// but it tests the code path
	if err != nil {
		t.Logf("Expected error due to missing profile: %v", err)
	}
	
	if result != nil {
		t.Logf("Got result: IsError=%v", result.IsError)
	}
}
