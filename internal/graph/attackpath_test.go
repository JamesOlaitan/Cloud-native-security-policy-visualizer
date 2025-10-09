package graph

import (
	"testing"

	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

func TestFindAttackPath(t *testing.T) {
	g := New()

	// Add nodes
	principal := ingest.Node{
		ID:     "arn:aws:iam::123456789012:role/DevRole",
		Kind:   ingest.PRINCIPAL,
		Labels: []string{"aws", "role"},
		Props: map[string]string{
			"name": "DevRole",
		},
	}

	policy := ingest.Node{
		ID:     "arn:aws:iam::123456789012:policy/DevDataAccess",
		Kind:   ingest.POLICY,
		Labels: []string{"aws", "policy"},
		Props: map[string]string{
			"name": "DevDataAccess",
		},
	}

	resource := ingest.Node{
		ID:     "arn:aws:s3:::data-bkt",
		Kind:   ingest.RESOURCE,
		Labels: []string{"aws", "s3"},
		Props: map[string]string{
			"name": "data-bkt",
		},
	}

	g.AddNode(principal)
	g.AddNode(policy)
	g.AddNode(resource)

	// Add edges
	attachEdge := ingest.Edge{
		Src:   principal.ID,
		Dst:   policy.ID,
		Kind:  "HAS_POLICY",
		Props: map[string]string{},
	}

	allowsEdge := ingest.Edge{
		Src:  policy.ID,
		Dst:  resource.ID,
		Kind: "ALLOWS_ACCESS",
		Props: map[string]string{
			"action": "s3:GetObject",
		},
	}

	if err := g.AddEdge(attachEdge); err != nil {
		t.Fatalf("Failed to add attach edge: %v", err)
	}
	if err := g.AddEdge(allowsEdge); err != nil {
		t.Fatalf("Failed to add allows edge: %v", err)
	}

	// Test finding path with explicit target
	t.Run("Find path to explicit target", func(t *testing.T) {
		result, err := g.FindAttackPath(principal.ID, resource.ID, nil, 8)
		if err != nil {
			t.Fatalf("FindAttackPath failed: %v", err)
		}

		if !result.Found {
			t.Fatal("Expected to find path")
		}

		if len(result.Nodes) != 3 {
			t.Errorf("Expected 3 nodes in path, got %d", len(result.Nodes))
		}

		if len(result.Edges) != 2 {
			t.Errorf("Expected 2 edges in path, got %d", len(result.Edges))
		}

		// Verify path order
		if result.Nodes[0].ID != principal.ID {
			t.Errorf("Expected first node to be %s, got %s", principal.ID, result.Nodes[0].ID)
		}
		if result.Nodes[2].ID != resource.ID {
			t.Errorf("Expected last node to be %s, got %s", resource.ID, result.Nodes[2].ID)
		}
	})

	// Test maxHops limit
	t.Run("Respect maxHops limit", func(t *testing.T) {
		result, err := g.FindAttackPath(principal.ID, resource.ID, nil, 1)
		if err != nil {
			t.Fatalf("FindAttackPath failed: %v", err)
		}

		if result.Found {
			t.Error("Expected no path with maxHops=1 (path needs 2 hops)")
		}
	})

	// Test no path found
	t.Run("No path to non-existent node", func(t *testing.T) {
		_, err := g.FindAttackPath(principal.ID, "arn:aws:s3:::nonexistent", nil, 8)
		if err == nil {
			t.Fatal("Expected error for non-existent target")
		}
	})

	// Test sensitive resource tagging
	t.Run("Find path to sensitive resource", func(t *testing.T) {
		// Mark resource as sensitive
		if err := g.MarkSensitive(resource.ID); err != nil {
			t.Fatalf("Failed to mark resource as sensitive: %v", err)
		}

		// Find path without explicit target, using sensitive tag
		result, err := g.FindAttackPath(principal.ID, "", []string{"sensitive"}, 8)
		if err != nil {
			t.Fatalf("FindAttackPath failed: %v", err)
		}

		if !result.Found {
			t.Fatal("Expected to find path to sensitive resource")
		}

		if result.Nodes[len(result.Nodes)-1].ID != resource.ID {
			t.Errorf("Expected to find path to sensitive resource %s", resource.ID)
		}
	})
}

func TestFindNearestSensitiveResource(t *testing.T) {
	g := New()

	// Add principal
	principal := ingest.Node{
		ID:     "arn:aws:iam::123456789012:role/DevRole",
		Kind:   ingest.PRINCIPAL,
		Labels: []string{"aws", "role"},
		Props:  map[string]string{},
	}
	g.AddNode(principal)

	// Add two resources at different distances
	nearResource := ingest.Node{
		ID:     "arn:aws:s3:::near-bkt",
		Kind:   ingest.RESOURCE,
		Labels: []string{"aws", "s3"},
		Props: map[string]string{
			"sensitive": "true",
		},
	}

	farResource := ingest.Node{
		ID:     "arn:aws:s3:::far-bkt",
		Kind:   ingest.RESOURCE,
		Labels: []string{"aws", "s3"},
		Props: map[string]string{
			"sensitive": "true",
		},
	}

	middlePolicy := ingest.Node{
		ID:     "arn:aws:iam::123456789012:policy/AccessPolicy",
		Kind:   ingest.POLICY,
		Labels: []string{"aws", "policy"},
		Props:  map[string]string{},
	}

	g.AddNode(nearResource)
	g.AddNode(farResource)
	g.AddNode(middlePolicy)

	// Create paths
	// Principal -> nearResource (1 hop)
	if err := g.AddEdge(ingest.Edge{
		Src:   principal.ID,
		Dst:   nearResource.ID,
		Kind:  "DIRECT_ACCESS",
		Props: map[string]string{},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	// Principal -> middlePolicy -> farResource (2 hops)
	if err := g.AddEdge(ingest.Edge{
		Src:   principal.ID,
		Dst:   middlePolicy.ID,
		Kind:  "HAS_POLICY",
		Props: map[string]string{},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}
	if err := g.AddEdge(ingest.Edge{
		Src:   middlePolicy.ID,
		Dst:   farResource.ID,
		Kind:  "ALLOWS_ACCESS",
		Props: map[string]string{},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	// Test that nearest is found
	result, err := g.FindAttackPath(principal.ID, "", []string{"sensitive"}, 8)
	if err != nil {
		t.Fatalf("FindAttackPath failed: %v", err)
	}

	if !result.Found {
		t.Fatal("Expected to find path to sensitive resource")
	}

	// Should find nearest (1-hop path)
	if len(result.Nodes) != 2 {
		t.Errorf("Expected 2 nodes (nearest path), got %d", len(result.Nodes))
	}

	if result.Nodes[len(result.Nodes)-1].ID != nearResource.ID {
		t.Errorf("Expected to find nearest resource %s, got %s",
			nearResource.ID, result.Nodes[len(result.Nodes)-1].ID)
	}
}

func TestFindSensitiveResources(t *testing.T) {
	g := New()

	// Add mix of sensitive and non-sensitive resources
	sensitive1 := ingest.Node{
		ID:     "arn:aws:s3:::sensitive-bkt",
		Kind:   ingest.RESOURCE,
		Labels: []string{"aws", "s3"},
		Props: map[string]string{
			"sensitive": "true",
		},
	}

	sensitive2 := ingest.Node{
		ID:     "arn:aws:s3:::another-sensitive-bkt",
		Kind:   ingest.RESOURCE,
		Labels: []string{"aws", "s3"},
		Props: map[string]string{
			"sensitive": "true",
		},
	}

	normal := ingest.Node{
		ID:     "arn:aws:s3:::normal-bkt",
		Kind:   ingest.RESOURCE,
		Labels: []string{"aws", "s3"},
		Props:  map[string]string{},
	}

	g.AddNode(sensitive1)
	g.AddNode(sensitive2)
	g.AddNode(normal)

	resources := g.findSensitiveResources()

	if len(resources) != 2 {
		t.Errorf("Expected 2 sensitive resources, got %d", len(resources))
	}

	// Check they're sorted for determinism
	if len(resources) == 2 {
		if resources[0] > resources[1] {
			t.Error("Expected sensitive resources to be sorted")
		}
	}
}

func TestMarkSensitive(t *testing.T) {
	g := New()

	node := ingest.Node{
		ID:     "arn:aws:s3:::test-bkt",
		Kind:   ingest.RESOURCE,
		Labels: []string{"aws", "s3"},
		Props:  map[string]string{},
	}
	g.AddNode(node)

	// Mark as sensitive
	if err := g.MarkSensitive(node.ID); err != nil {
		t.Fatalf("MarkSensitive failed: %v", err)
	}

	// Verify it's marked
	marked, ok := g.GetNode(node.ID)
	if !ok {
		t.Fatal("Failed to get node")
	}

	if marked.Props["sensitive"] != "true" {
		t.Error("Expected node to be marked as sensitive")
	}

	// Test marking non-existent node
	if err := g.MarkSensitive("arn:aws:s3:::nonexistent"); err == nil {
		t.Error("Expected error when marking non-existent node")
	}
}
