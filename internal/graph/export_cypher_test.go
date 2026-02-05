package graph

import (
	"strings"
	"testing"

	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

func TestExportCypher(t *testing.T) {
	g := New()

	// Add nodes
	principal := ingest.Node{
		ID:     "arn:aws:iam::123456789012:role/DevRole",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"aws", "role"},
		Props: map[string]string{
			"name": "DevRole",
		},
	}

	resource := ingest.Node{
		ID:     "arn:aws:s3:::data-bkt",
		Kind:   ingest.KindResource,
		Labels: []string{"aws", "s3"},
		Props: map[string]string{
			"name":      "data-bkt",
			"sensitive": "true",
		},
	}

	g.AddNode(principal)
	g.AddNode(resource)

	// Add edge
	if err := g.AddEdge(ingest.Edge{
		Src:  principal.ID,
		Dst:  resource.ID,
		Kind: "HAS_ACCESS",
		Props: map[string]string{
			"action": "s3:GetObject",
		},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	// Export to Cypher
	cypher, err := g.ExportCypher()
	if err != nil {
		t.Fatalf("ExportCypher failed: %v", err)
	}

	// Verify content
	expectedStrings := []string{
		"// AccessGraph Neo4j Export",
		"CREATE CONSTRAINT node_id IF NOT EXISTS",
		"CREATE INDEX node_kind_idx IF NOT EXISTS",
		"// ========== NODES ==========",
		"// ========== EDGES ==========",
		"MERGE (n:Node:K_PRINCIPAL",
		"MERGE (n:Node:K_RESOURCE",
		"MATCH (a:Node {id:",
		"MERGE (a)-[r:K_HAS_ACCESS]->(b)",
		`"id":"arn:aws:iam::123456789012:role/DevRole"`,
		`"id":"arn:aws:s3:::data-bkt"`,
		`"kind":"PRINCIPAL"`,
		`"kind":"RESOURCE"`,
		`"action":"s3:GetObject"`,
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(cypher, expected) {
			t.Errorf("Expected Cypher to contain %q", expected)
		}
	}

	// Verify nodes come before edges
	nodesIdx := strings.Index(cypher, "// ========== NODES ==========")
	edgesIdx := strings.Index(cypher, "// ========== EDGES ==========")
	if nodesIdx == -1 || edgesIdx == -1 || nodesIdx >= edgesIdx {
		t.Error("Nodes should come before edges in export")
	}
}

func TestExportCypherDeterministic(t *testing.T) {
	// Create two identical graphs
	g1 := New()
	g2 := New()

	nodes := []ingest.Node{
		{
			ID:     "node1",
			Kind:   ingest.KindPrincipal,
			Labels: []string{"test"},
			Props:  map[string]string{"a": "1"},
		},
		{
			ID:     "node2",
			Kind:   ingest.KindResource,
			Labels: []string{"test"},
			Props:  map[string]string{"b": "2"},
		},
	}

	edges := []ingest.Edge{
		{Src: "node1", Dst: "node2", Kind: "EDGE1", Props: map[string]string{}},
	}

	for _, node := range nodes {
		g1.AddNode(node)
		g2.AddNode(node)
	}

	for _, edge := range edges {
		if err := g1.AddEdge(edge); err != nil {
			t.Fatalf("Failed to add edge to g1: %v", err)
		}
		if err := g2.AddEdge(edge); err != nil {
			t.Fatalf("Failed to add edge to g2: %v", err)
		}
	}

	// Export both
	cypher1, err1 := g1.ExportCypher()
	cypher2, err2 := g2.ExportCypher()

	if err1 != nil || err2 != nil {
		t.Fatalf("ExportCypher failed: %v, %v", err1, err2)
	}

	// Should be identical
	if cypher1 != cypher2 {
		t.Error("Expected identical exports for identical graphs")
	}
}

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"PRINCIPAL", "PRINCIPAL"},
		{"HAS_POLICY", "HAS_POLICY"},
		{"ALLOWS-ACCESS", "ALLOWS_ACCESS"},
		{"foo:bar", "foo_bar"},
		{"test.label", "test_label"},
		{"123abc", "123abc"},
		{"foo@bar#baz", "foo_bar_baz"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeLabel(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeLabel(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestQuoteString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", `"simple"`},
		{`with"quote`, `"with\"quote"`},
		{`with\backslash`, `"with\\backslash"`},
		{`both\"together`, `"both\\\"together"`},
		{"arn:aws:iam::123:role/Test", `"arn:aws:iam::123:role/Test"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := quoteString(tt.input)
			if result != tt.expected {
				t.Errorf("quoteString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExportCypherEmpty(t *testing.T) {
	g := New()
	cypher, err := g.ExportCypher()

	if err != nil {
		t.Fatalf("ExportCypher failed: %v", err)
	}

	// Should still have header and constraints
	if !strings.Contains(cypher, "CREATE CONSTRAINT") {
		t.Error("Expected constraint even for empty graph")
	}

	// Should not have any MERGE statements
	if strings.Count(cypher, "MERGE") > 0 {
		t.Error("Expected no MERGE statements for empty graph")
	}
}

func TestExportCypherWithSpecialCharacters(t *testing.T) {
	g := New()

	// Add node with special characters in ID
	node := ingest.Node{
		ID:     `arn:aws:s3:::bucket-with-"quotes"-and-\backslashes`,
		Kind:   ingest.KindResource,
		Labels: []string{"aws"},
		Props:  map[string]string{},
	}

	g.AddNode(node)

	cypher, err := g.ExportCypher()
	if err != nil {
		t.Fatalf("ExportCypher failed: %v", err)
	}

	// Should properly escape quotes and backslashes
	if !strings.Contains(cypher, `\"`) {
		t.Error("Expected escaped quotes in Cypher output")
	}
}
