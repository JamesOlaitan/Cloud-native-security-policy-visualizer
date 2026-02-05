package reco

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

func TestRecommend(t *testing.T) {
	g := graph.New()

	// Create a scenario with a wildcard policy
	principal := ingest.Node{
		ID:     "arn:aws:iam::123456789012:role/DevRole",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"aws", "role"},
		Props:  map[string]string{},
	}

	wildcardPolicy := ingest.Node{
		ID:     "arn:aws:iam::123456789012:policy/DevDataAccess",
		Kind:   ingest.KindPolicy,
		Labels: []string{"aws", "policy"},
		Props: map[string]string{
			"name":   "DevDataAccess",
			"action": "*", // Wildcard!
		},
	}

	resource1 := ingest.Node{
		ID:     "arn:aws:s3:::data-bkt",
		Kind:   ingest.KindResource,
		Labels: []string{"aws", "s3"},
		Props: map[string]string{
			"name": "data-bkt",
		},
	}

	resource2 := ingest.Node{
		ID:     "arn:aws:s3:::logs-bkt",
		Kind:   ingest.KindResource,
		Labels: []string{"aws", "s3"},
		Props: map[string]string{
			"name": "logs-bkt",
		},
	}

	g.AddNode(principal)
	g.AddNode(wildcardPolicy)
	g.AddNode(resource1)
	g.AddNode(resource2)

	// Create edges: principal -> policy -> resources
	if err := g.AddEdge(ingest.Edge{
		Src:   principal.ID,
		Dst:   wildcardPolicy.ID,
		Kind:  ingest.EdgeAttachedPolicy,
		Props: map[string]string{},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	if err := g.AddEdge(ingest.Edge{
		Src:  wildcardPolicy.ID,
		Dst:  resource1.ID,
		Kind: "ALLOWS_ACCESS",
		Props: map[string]string{
			"action": "s3:GetObject",
		},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	if err := g.AddEdge(ingest.Edge{
		Src:  wildcardPolicy.ID,
		Dst:  resource2.ID,
		Kind: "ALLOWS_ACCESS",
		Props: map[string]string{
			"action": "s3:PutObject",
		},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	// Create recommender
	recommender := New(g)

	// Get recommendation
	rec, err := recommender.Recommend(wildcardPolicy.ID, "", nil, 20)
	if err != nil {
		t.Fatalf("Recommend failed: %v", err)
	}

	// Verify recommendation
	if rec.PolicyID != wildcardPolicy.ID {
		t.Errorf("Expected policyId %s, got %s", wildcardPolicy.ID, rec.PolicyID)
	}

	// Should suggest specific actions
	if len(rec.SuggestedActions) == 0 {
		t.Error("Expected suggested actions")
	}

	// Should not include wildcard in suggestions
	for _, action := range rec.SuggestedActions {
		if action == "*" || strings.HasSuffix(action, ":*") {
			t.Errorf("Suggested actions should not include wildcards, got: %s", action)
		}
	}

	// Should suggest resources
	if len(rec.SuggestedResources) == 0 {
		t.Error("Expected suggested resources")
	}

	// Verify actions are sorted
	if len(rec.SuggestedActions) >= 2 {
		if rec.SuggestedActions[0] > rec.SuggestedActions[1] {
			t.Error("Expected suggested actions to be sorted")
		}
	}

	// Verify patch JSON is valid
	var patches []map[string]interface{}
	if err := json.Unmarshal([]byte(rec.PatchJSON), &patches); err != nil {
		t.Errorf("Invalid patch JSON: %v", err)
	}

	if len(patches) == 0 {
		t.Error("Expected at least one patch operation")
	}

	// Verify rationale is present
	if rec.Rationale == "" {
		t.Error("Expected rationale to be non-empty")
	}

	if !strings.Contains(rec.Rationale, "wildcard") {
		t.Error("Expected rationale to mention wildcards")
	}
}

func TestRecommendWithTarget(t *testing.T) {
	g := graph.New()

	principal := ingest.Node{
		ID:     "arn:aws:iam::123456789012:role/DevRole",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"aws", "role"},
		Props:  map[string]string{},
	}

	policy := ingest.Node{
		ID:     "arn:aws:iam::123456789012:policy/DevPolicy",
		Kind:   ingest.KindPolicy,
		Labels: []string{"aws", "policy"},
		Props: map[string]string{
			"action": "*",
		},
	}

	targetResource := ingest.Node{
		ID:     "arn:aws:s3:::target-bkt",
		Kind:   ingest.KindResource,
		Labels: []string{"aws", "s3"},
		Props:  map[string]string{},
	}

	otherResource := ingest.Node{
		ID:     "arn:aws:s3:::other-bkt",
		Kind:   ingest.KindResource,
		Labels: []string{"aws", "s3"},
		Props:  map[string]string{},
	}

	g.AddNode(principal)
	g.AddNode(policy)
	g.AddNode(targetResource)
	g.AddNode(otherResource)

	// Create paths
	if err := g.AddEdge(ingest.Edge{
		Src:   principal.ID,
		Dst:   policy.ID,
		Kind:  ingest.EdgeAttachedPolicy,
		Props: map[string]string{},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	if err := g.AddEdge(ingest.Edge{
		Src:  policy.ID,
		Dst:  targetResource.ID,
		Kind: "ALLOWS_ACCESS",
		Props: map[string]string{
			"action": "s3:GetObject",
		},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	if err := g.AddEdge(ingest.Edge{
		Src:  policy.ID,
		Dst:  otherResource.ID,
		Kind: "ALLOWS_ACCESS",
		Props: map[string]string{
			"action": "s3:DeleteObject",
		},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	recommender := New(g)

	// Recommend with specific target
	rec, err := recommender.Recommend(policy.ID, targetResource.ID, nil, 20)
	if err != nil {
		t.Fatalf("Recommend failed: %v", err)
	}

	// Should only suggest the target resource
	if len(rec.SuggestedResources) != 1 {
		t.Errorf("Expected 1 suggested resource (target only), got %d", len(rec.SuggestedResources))
	}

	if len(rec.SuggestedResources) > 0 && rec.SuggestedResources[0] != targetResource.ID {
		t.Errorf("Expected target resource %s, got %s", targetResource.ID, rec.SuggestedResources[0])
	}
}

func TestRecommendWithCap(t *testing.T) {
	g := graph.New()

	principal := ingest.Node{
		ID:     "arn:aws:iam::123456789012:role/DevRole",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"aws", "role"},
		Props:  map[string]string{},
	}

	policy := ingest.Node{
		ID:     "arn:aws:iam::123456789012:policy/DevPolicy",
		Kind:   ingest.KindPolicy,
		Labels: []string{"aws", "policy"},
		Props: map[string]string{
			"action": "*",
		},
	}

	g.AddNode(principal)
	g.AddNode(policy)

	// Add many resources
	for i := 0; i < 30; i++ {
		resource := ingest.Node{
			ID:     fmt.Sprintf("arn:aws:s3:::bucket-%d", i),
			Kind:   ingest.KindResource,
			Labels: []string{"aws", "s3"},
			Props:  map[string]string{},
		}
		g.AddNode(resource)

		if err := g.AddEdge(ingest.Edge{
			Src:  policy.ID,
			Dst:  resource.ID,
			Kind: "ALLOWS_ACCESS",
			Props: map[string]string{
				"action": fmt.Sprintf("s3:Action%d", i),
			},
		}); err != nil {
			t.Fatalf("Failed to add edge: %v", err)
		}
	}

	if err := g.AddEdge(ingest.Edge{
		Src:   principal.ID,
		Dst:   policy.ID,
		Kind:  ingest.EdgeAttachedPolicy,
		Props: map[string]string{},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	recommender := New(g)

	// Recommend with cap of 10
	rec, err := recommender.Recommend(policy.ID, "", nil, 10)
	if err != nil {
		t.Fatalf("Recommend failed: %v", err)
	}

	// Should respect cap
	if len(rec.SuggestedActions) > 10 {
		t.Errorf("Expected max 10 actions (cap), got %d", len(rec.SuggestedActions))
	}

	if len(rec.SuggestedResources) > 10 {
		t.Errorf("Expected max 10 resources (cap), got %d", len(rec.SuggestedResources))
	}
}

func TestRecommendNoWildcard(t *testing.T) {
	g := graph.New()

	principal := ingest.Node{
		ID:     "arn:aws:iam::123456789012:role/DevRole",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"aws", "role"},
		Props:  map[string]string{},
	}

	normalPolicy := ingest.Node{
		ID:     "arn:aws:iam::123456789012:policy/NormalPolicy",
		Kind:   ingest.KindPolicy,
		Labels: []string{"aws", "policy"},
		Props: map[string]string{
			"name": "NormalPolicy",
			// No wildcard
		},
	}

	g.AddNode(principal)
	g.AddNode(normalPolicy)

	if err := g.AddEdge(ingest.Edge{
		Src:   principal.ID,
		Dst:   normalPolicy.ID,
		Kind:  ingest.EdgeAttachedPolicy,
		Props: map[string]string{},
	}); err != nil {
		t.Fatalf("Failed to add edge: %v", err)
	}

	recommender := New(g)

	// Recommend for non-wildcard policy
	rec, err := recommender.Recommend(normalPolicy.ID, "", nil, 20)
	if err != nil {
		t.Fatalf("Recommend failed: %v", err)
	}

	// Should indicate no wildcards
	if !strings.Contains(rec.Rationale, "does not contain wildcard") {
		t.Error("Expected rationale to indicate no wildcards")
	}
}

func TestHasWildcard(t *testing.T) {
	tests := []struct {
		name     string
		policy   ingest.Node
		expected bool
	}{
		{
			name: "Wildcard action *",
			policy: ingest.Node{
				Kind: ingest.KindPolicy,
				Props: map[string]string{
					"action": "*",
				},
			},
			expected: true,
		},
		{
			name: "Wildcard action service:*",
			policy: ingest.Node{
				Kind: ingest.KindPolicy,
				Props: map[string]string{
					"action": "s3:*",
				},
			},
			expected: true,
		},
		{
			name: "Wildcard resource",
			policy: ingest.Node{
				Kind: ingest.KindPolicy,
				Props: map[string]string{
					"resource": "arn:aws:s3:::*/*",
				},
			},
			expected: true,
		},
		{
			name: "No wildcard",
			policy: ingest.Node{
				Kind: ingest.KindPolicy,
				Props: map[string]string{
					"action": "s3:GetObject",
				},
			},
			expected: false,
		},
		{
			name: "Empty props",
			policy: ingest.Node{
				Kind:  ingest.KindPolicy,
				Props: map[string]string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasWildcard(tt.policy)
			if result != tt.expected {
				t.Errorf("hasWildcard() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGenerateJSONPatch(t *testing.T) {
	g := graph.New()
	recommender := New(g)

	policy := ingest.Node{
		ID:    "test-policy",
		Kind:  ingest.KindPolicy,
		Props: map[string]string{},
	}

	actions := []string{"s3:GetObject", "s3:PutObject"}
	resources := []string{"arn:aws:s3:::bucket1", "arn:aws:s3:::bucket2"}

	patches := recommender.generateJSONPatch(policy, actions, resources)

	if len(patches) != 2 {
		t.Errorf("Expected 2 patches, got %d", len(patches))
	}

	// Verify structure
	for _, patch := range patches {
		if patch["op"] != "replace" {
			t.Errorf("Expected op 'replace', got %v", patch["op"])
		}

		path, ok := patch["path"].(string)
		if !ok {
			t.Error("Expected path to be string")
		}

		if !strings.Contains(path, "Statement") {
			t.Errorf("Expected path to contain 'Statement', got %s", path)
		}
	}
}
