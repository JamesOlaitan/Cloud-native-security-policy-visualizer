package graph

import (
	"testing"

	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

func TestGraphOperations(t *testing.T) {
	g := New()

	// Add nodes
	node1 := ingest.Node{
		ID:     "node1",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"test-principal"},
		Props:  map[string]string{"name": "test"},
	}

	node2 := ingest.Node{
		ID:     "node2",
		Kind:   ingest.KindPolicy,
		Labels: []string{"test-policy"},
		Props:  map[string]string{"name": "policy"},
	}

	node3 := ingest.Node{
		ID:     "node3",
		Kind:   ingest.KindResource,
		Labels: []string{"test-resource"},
		Props:  map[string]string{"name": "resource"},
	}

	g.AddNode(node1)
	g.AddNode(node2)
	g.AddNode(node3)

	// Test GetNode
	retrieved, ok := g.GetNode("node1")
	if !ok {
		t.Error("Expected to find node1")
	}
	if retrieved.ID != "node1" {
		t.Errorf("Expected ID node1, got %s", retrieved.ID)
	}

	// Add edges
	edge1 := ingest.Edge{
		Src:   "node1",
		Dst:   "node2",
		Kind:  ingest.EdgeAttachedPolicy,
		Props: map[string]string{},
	}

	edge2 := ingest.Edge{
		Src:   "node2",
		Dst:   "node3",
		Kind:  ingest.EdgeAppliesTo,
		Props: map[string]string{},
	}

	if err := g.AddEdge(edge1); err != nil {
		t.Errorf("Failed to add edge1: %v", err)
	}

	if err := g.AddEdge(edge2); err != nil {
		t.Errorf("Failed to add edge2: %v", err)
	}

	// Test GetNodes
	nodes := g.GetNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	// Test GetEdges
	edges := g.GetEdges()
	if len(edges) != 2 {
		t.Errorf("Expected 2 edges, got %d", len(edges))
	}

	// Test GetNeighbors
	neighbors, edgeKinds, err := g.GetNeighbors("node1", nil)
	if err != nil {
		t.Errorf("GetNeighbors failed: %v", err)
	}
	if len(neighbors) == 0 {
		t.Error("Expected neighbors")
	}
	if len(edgeKinds) == 0 {
		t.Error("Expected edge kinds")
	}

	// Test ShortestPath
	pathNodes, pathEdges, err := g.ShortestPath("node1", "node3", 10)
	if err != nil {
		t.Errorf("ShortestPath failed: %v", err)
	}
	if len(pathNodes) < 2 {
		t.Errorf("Expected path with at least 2 nodes, got %d", len(pathNodes))
	}
	if len(pathEdges) < 1 {
		t.Errorf("Expected path with at least 1 edge, got %d", len(pathEdges))
	}

	// Test path from node1 to node3
	if pathNodes[0].ID != "node1" || pathNodes[len(pathNodes)-1].ID != "node3" {
		t.Error("Path should start at node1 and end at node3")
	}
}

func TestGraphMissingNode(t *testing.T) {
	g := New()

	node1 := ingest.Node{ID: "node1", Kind: ingest.KindPrincipal, Labels: []string{}, Props: map[string]string{}}
	g.AddNode(node1)

	// Try to add edge with missing destination
	edge := ingest.Edge{Src: "node1", Dst: "missing", Kind: "TEST", Props: map[string]string{}}
	err := g.AddEdge(edge)
	if err == nil {
		t.Error("Expected error when adding edge with missing node")
	}

	// Try to get missing node
	_, ok := g.GetNode("missing")
	if ok {
		t.Error("Should not find missing node")
	}

	// Try path to missing node
	node2 := ingest.Node{ID: "node2", Kind: ingest.KindResource, Labels: []string{}, Props: map[string]string{}}
	g.AddNode(node2)

	_, _, err = g.ShortestPath("node1", "node2", 8)
	if err == nil {
		t.Error("Expected error for path with no connection")
	}
}
