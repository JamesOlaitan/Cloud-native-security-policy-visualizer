package graph

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

func TestExportSARIFAttackPath(t *testing.T) {
	nodes := []ingest.Node{
		{
			ID:     "arn:aws:iam::123456789012:role/DevRole",
			Kind:   ingest.PRINCIPAL,
			Labels: []string{"aws", "role"},
			Props:  map[string]string{},
		},
		{
			ID:     "arn:aws:iam::123456789012:policy/DevDataAccess",
			Kind:   ingest.POLICY,
			Labels: []string{"aws", "policy"},
			Props:  map[string]string{},
		},
		{
			ID:     "arn:aws:s3:::data-bkt",
			Kind:   ingest.RESOURCE,
			Labels: []string{"aws", "s3"},
			Props: map[string]string{
				"sensitive": "true",
			},
		},
	}

	edges := []ingest.Edge{
		{
			Src:   "arn:aws:iam::123456789012:role/DevRole",
			Dst:   "arn:aws:iam::123456789012:policy/DevDataAccess",
			Kind:  "HAS_POLICY",
			Props: map[string]string{},
		},
		{
			Src:  "arn:aws:iam::123456789012:policy/DevDataAccess",
			Dst:  "arn:aws:s3:::data-bkt",
			Kind: "ALLOWS_ACCESS",
			Props: map[string]string{
				"action": "s3:GetObject",
			},
		},
	}

	sarifJSON, err := ExportSARIFAttackPath(
		"arn:aws:iam::123456789012:role/DevRole",
		"arn:aws:s3:::data-bkt",
		nodes,
		edges,
	)

	if err != nil {
		t.Fatalf("ExportSARIFAttackPath failed: %v", err)
	}

	// Parse JSON to verify structure
	var sarif SARIF
	if err := json.Unmarshal([]byte(sarifJSON), &sarif); err != nil {
		t.Fatalf("Failed to parse SARIF JSON: %v", err)
	}

	// Verify version
	if sarif.Version != "2.1.0" {
		t.Errorf("Expected version 2.1.0, got %s", sarif.Version)
	}

	// Verify schema
	if !strings.Contains(sarif.Schema, "sarif-2.1.0") {
		t.Errorf("Expected SARIF 2.1.0 schema, got %s", sarif.Schema)
	}

	// Verify runs
	if len(sarif.Runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(sarif.Runs))
	}

	run := sarif.Runs[0]

	// Verify tool
	if run.Tool.Driver.Name != "AccessGraph" {
		t.Errorf("Expected tool name AccessGraph, got %s", run.Tool.Driver.Name)
	}

	if run.Tool.Driver.Version != "1.1.0" {
		t.Errorf("Expected version 1.1.0, got %s", run.Tool.Driver.Version)
	}

	// Verify rules (should have 2 unique edge kinds)
	if len(run.Tool.Driver.Rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(run.Tool.Driver.Rules))
	}

	// Verify results (one per hop, so 2 results)
	if len(run.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(run.Results))
	}

	// Verify first result
	firstResult := run.Results[0]
	if firstResult.RuleID != "attack-path/HAS_POLICY" {
		t.Errorf("Expected ruleId attack-path/HAS_POLICY, got %s", firstResult.RuleID)
	}

	if firstResult.Level != "warning" {
		t.Errorf("Expected level warning, got %s", firstResult.Level)
	}

	if !strings.Contains(firstResult.Message.Text, "Step 1") {
		t.Error("Expected message to contain 'Step 1'")
	}

	// Verify locations
	if len(firstResult.Locations) != 1 {
		t.Errorf("Expected 1 location, got %d", len(firstResult.Locations))
	}

	if firstResult.Locations[0].PhysicalLocation.Region.StartLine != 1 {
		t.Errorf("Expected StartLine 1, got %d", firstResult.Locations[0].PhysicalLocation.Region.StartLine)
	}
}

func TestExportSARIFWithCriticalEdges(t *testing.T) {
	nodes := []ingest.Node{
		{
			ID:     "arn:aws:iam::111111111111:role/DevRole",
			Kind:   ingest.PRINCIPAL,
			Labels: []string{"aws", "role"},
			Props:  map[string]string{},
		},
		{
			ID:     "arn:aws:iam::222222222222:role/TargetRole",
			Kind:   ingest.PRINCIPAL,
			Labels: []string{"aws", "role"},
			Props:  map[string]string{},
		},
	}

	edges := []ingest.Edge{
		{
			Src:  "arn:aws:iam::111111111111:role/DevRole",
			Dst:  "arn:aws:iam::222222222222:role/TargetRole",
			Kind: "ASSUMES_ROLE",
			Props: map[string]string{
				"cross_account": "true",
			},
		},
	}

	sarifJSON, err := ExportSARIFAttackPath(
		"arn:aws:iam::111111111111:role/DevRole",
		"arn:aws:iam::222222222222:role/TargetRole",
		nodes,
		edges,
	)

	if err != nil {
		t.Fatalf("ExportSARIFAttackPath failed: %v", err)
	}

	var sarif SARIF
	if err := json.Unmarshal([]byte(sarifJSON), &sarif); err != nil {
		t.Fatalf("Failed to parse SARIF JSON: %v", err)
	}

	// Cross-account should be marked as error
	if sarif.Runs[0].Results[0].Level != "error" {
		t.Errorf("Expected level error for cross-account, got %s", sarif.Runs[0].Results[0].Level)
	}
}

func TestExportSARIFWithWildcard(t *testing.T) {
	nodes := []ingest.Node{
		{
			ID:     "arn:aws:iam::123456789012:role/AdminRole",
			Kind:   ingest.PRINCIPAL,
			Labels: []string{"aws", "role"},
			Props:  map[string]string{},
		},
		{
			ID:     "arn:aws:s3:::prod-data",
			Kind:   ingest.RESOURCE,
			Labels: []string{"aws", "s3"},
			Props:  map[string]string{},
		},
	}

	edges := []ingest.Edge{
		{
			Src:  "arn:aws:iam::123456789012:role/AdminRole",
			Dst:  "arn:aws:s3:::prod-data",
			Kind: "ALLOWS_ACCESS",
			Props: map[string]string{
				"action": "*",
			},
		},
	}

	sarifJSON, err := ExportSARIFAttackPath(
		"arn:aws:iam::123456789012:role/AdminRole",
		"arn:aws:s3:::prod-data",
		nodes,
		edges,
	)

	if err != nil {
		t.Fatalf("ExportSARIFAttackPath failed: %v", err)
	}

	var sarif SARIF
	if err := json.Unmarshal([]byte(sarifJSON), &sarif); err != nil {
		t.Fatalf("Failed to parse SARIF JSON: %v", err)
	}

	// Wildcard should be marked as error
	if sarif.Runs[0].Results[0].Level != "error" {
		t.Errorf("Expected level error for wildcard, got %s", sarif.Runs[0].Results[0].Level)
	}

	// Message should include action
	if !strings.Contains(sarif.Runs[0].Results[0].Message.Text, "[Action: *]") {
		t.Error("Expected message to contain [Action: *]")
	}
}

func TestGenerateStableURI(t *testing.T) {
	// Test that URIs are stable and deterministic
	uri1 := generateStableURI("node1", "node2")
	uri2 := generateStableURI("node1", "node2")
	uri3 := generateStableURI("node2", "node1")

	if uri1 != uri2 {
		t.Error("Expected URIs to be stable for same inputs")
	}

	if uri1 == uri3 {
		t.Error("Expected different URIs for different node pairs")
	}

	if !strings.HasPrefix(uri1, "accessgraph://path/") {
		t.Errorf("Expected URI to start with accessgraph://path/, got %s", uri1)
	}
}

func TestIsCriticalEdge(t *testing.T) {
	tests := []struct {
		name     string
		edge     ingest.Edge
		expected bool
	}{
		{
			name: "Cross-account is critical",
			edge: ingest.Edge{
				Kind: "ASSUMES_ROLE",
				Props: map[string]string{
					"cross_account": "true",
				},
			},
			expected: true,
		},
		{
			name: "Wildcard action is critical",
			edge: ingest.Edge{
				Kind: "ALLOWS_ACCESS",
				Props: map[string]string{
					"action": "*",
				},
			},
			expected: true,
		},
		{
			name: "Service wildcard is critical",
			edge: ingest.Edge{
				Kind: "ALLOWS_ACCESS",
				Props: map[string]string{
					"action": "s3:*",
				},
			},
			expected: true,
		},
		{
			name: "Normal permission not critical",
			edge: ingest.Edge{
				Kind: "ALLOWS_ACCESS",
				Props: map[string]string{
					"action": "s3:GetObject",
				},
			},
			expected: false,
		},
		{
			name: "No props not critical",
			edge: ingest.Edge{
				Kind:  "HAS_POLICY",
				Props: map[string]string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCriticalEdge(tt.edge)
			if result != tt.expected {
				t.Errorf("isCriticalEdge() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExportSARIFEmpty(t *testing.T) {
	_, err := ExportSARIFAttackPath("from", "to", nil, nil)
	if err == nil {
		t.Error("Expected error for empty nodes")
	}
}
