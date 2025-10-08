package store

import (
	"os"
	"testing"

	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

func TestStoreRoundTrip(t *testing.T) {
	// Create temp database
	tmpFile := t.TempDir() + "/test.db"
	defer os.Remove(tmpFile)

	store, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// Create a graph
	g := graph.New()

	node1 := ingest.Node{
		ID:     "test-node-1",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"label1", "label2"},
		Props:  map[string]string{"key": "value"},
	}

	node2 := ingest.Node{
		ID:     "test-node-2",
		Kind:   ingest.KindResource,
		Labels: []string{"resource"},
		Props:  map[string]string{"arn": "test-arn"},
	}

	g.AddNode(node1)
	g.AddNode(node2)

	edge := ingest.Edge{
		Src:   "test-node-1",
		Dst:   "test-node-2",
		Kind:  ingest.EdgeAppliesTo,
		Props: map[string]string{"prop": "val"},
	}

	g.AddEdge(edge)

	// Save snapshot
	err = store.SaveSnapshot("test-snapshot", "test-label", g)
	if err != nil {
		t.Fatalf("Failed to save snapshot: %v", err)
	}

	// Load snapshot
	loaded, err := store.LoadSnapshot("test-snapshot")
	if err != nil {
		t.Fatalf("Failed to load snapshot: %v", err)
	}

	// Verify nodes
	loadedNode, ok := loaded.GetNode("test-node-1")
	if !ok {
		t.Error("Failed to load node")
	}

	if loadedNode.Kind != ingest.KindPrincipal {
		t.Errorf("Expected kind PRINCIPAL, got %s", loadedNode.Kind)
	}

	if len(loadedNode.Labels) != 2 {
		t.Errorf("Expected 2 labels, got %d", len(loadedNode.Labels))
	}

	// Verify edges
	edges := loaded.GetEdges()
	if len(edges) != 1 {
		t.Errorf("Expected 1 edge, got %d", len(edges))
	}

	// List snapshots
	snapshots, err := store.ListSnapshots()
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if len(snapshots) == 0 {
		t.Error("Expected at least one snapshot")
	}

	// Get snapshot
	snap, err := store.GetSnapshot("test-snapshot")
	if err != nil {
		t.Fatalf("Failed to get snapshot: %v", err)
	}

	if snap.Label != "test-label" {
		t.Errorf("Expected label 'test-label', got '%s'", snap.Label)
	}

	// Count nodes and edges
	nodeCount, err := store.CountNodes("test-snapshot")
	if err != nil {
		t.Fatalf("Failed to count nodes: %v", err)
	}
	if nodeCount != 2 {
		t.Errorf("Expected 2 nodes, got %d", nodeCount)
	}

	edgeCount, err := store.CountEdges("test-snapshot")
	if err != nil {
		t.Fatalf("Failed to count edges: %v", err)
	}
	if edgeCount != 1 {
		t.Errorf("Expected 1 edge, got %d", edgeCount)
	}
}

func TestSearchPrincipals(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	defer os.Remove(tmpFile)

	store, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	g := graph.New()

	node := ingest.Node{
		ID:     "arn:aws:iam::123456789012:role/DevRole",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"DevRole", "aws-role"},
		Props:  map[string]string{"name": "DevRole"},
	}

	g.AddNode(node)

	err = store.SaveSnapshot("test", "test", g)
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Search
	results, err := store.SearchPrincipals("test", "DevRole", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results")
	}

	if results[0].ID != node.ID {
		t.Errorf("Expected ID %s, got %s", node.ID, results[0].ID)
	}
}
