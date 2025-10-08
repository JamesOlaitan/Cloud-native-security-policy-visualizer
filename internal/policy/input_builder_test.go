package policy

import (
	"testing"

	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

func TestBuildInput(t *testing.T) {
	g := graph.New()

	// Add AWS role
	role := ingest.Node{
		ID:     "arn:aws:iam::111111111111:role/TestRole",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"TestRole", "aws-role"},
		Props:  map[string]string{"name": "TestRole"},
	}
	g.AddNode(role)

	// Add cross-account trust
	account := ingest.Node{
		ID:     "arn:aws:iam::222222222222:root",
		Kind:   ingest.KindAccount,
		Labels: []string{"222222222222"},
		Props:  map[string]string{"account_id": "222222222222"},
	}
	g.AddNode(account)

	trustEdge := ingest.Edge{
		Src:   role.ID,
		Dst:   account.ID,
		Kind:  ingest.EdgeTrustsCrossAccount,
		Props: map[string]string{},
	}
	g.AddEdge(trustEdge)

	// Add policy with wildcard
	policy := ingest.Node{
		ID:     "arn:aws:iam::111111111111:policy/TestPolicy",
		Kind:   ingest.KindPolicy,
		Labels: []string{"TestPolicy"},
		Props:  map[string]string{"name": "TestPolicy"},
	}
	g.AddNode(policy)

	perm := ingest.Node{
		ID:     "perm1",
		Kind:   ingest.KindPerm,
		Labels: []string{"s3:*"},
		Props: map[string]string{
			"action":   "s3:*",
			"wildcard": "true",
		},
	}
	g.AddNode(perm)

	permEdge := ingest.Edge{
		Src:   policy.ID,
		Dst:   perm.ID,
		Kind:  ingest.EdgeAllowsAction,
		Props: map[string]string{},
	}
	g.AddEdge(permEdge)

	// Add K8s cluster-admin binding
	k8sRole := ingest.Node{
		ID:     "k8s:role:cluster-admin",
		Kind:   ingest.KindRole,
		Labels: []string{"cluster-admin"},
		Props: map[string]string{
			"name":          "cluster-admin",
			"cluster_admin": "true",
		},
	}
	g.AddNode(k8sRole)

	sa := ingest.Node{
		ID:     "k8s:sa:default:test-sa",
		Kind:   ingest.KindPrincipal,
		Labels: []string{"test-sa"},
		Props:  map[string]string{"name": "test-sa"},
	}
	g.AddNode(sa)

	bindingEdge := ingest.Edge{
		Src:  k8sRole.ID,
		Dst:  sa.ID,
		Kind: ingest.EdgeBindsTo,
		Props: map[string]string{
			"binding": "test-binding",
		},
	}
	g.AddEdge(bindingEdge)

	// Build input
	input := BuildInput(g)

	// Verify roles
	roles, ok := input["roles"].(map[string]interface{})
	if !ok || len(roles) == 0 {
		t.Error("Expected roles in input")
	}

	roleData, ok := roles[role.ID].(map[string]interface{})
	if !ok {
		t.Error("Expected role data")
	}

	trust, ok := roleData["trust"].(map[string]interface{})
	if !ok {
		t.Error("Expected trust data")
	}

	if trust["cross_account"] != true {
		t.Error("Expected cross_account to be true")
	}

	// Verify policies
	policies, ok := input["policies"].(map[string]interface{})
	if !ok || len(policies) == 0 {
		t.Error("Expected policies in input")
	}

	policyData, ok := policies[policy.ID].(map[string]interface{})
	if !ok {
		t.Error("Expected policy data")
	}

	if policyData["action_matches_wildcard"] != true {
		t.Error("Expected action_matches_wildcard to be true")
	}

	// Verify K8s bindings
	k8s, ok := input["k8s"].(map[string]interface{})
	if !ok {
		t.Error("Expected k8s in input")
	}

	bindings, ok := k8s["bindings"].(map[string]interface{})
	if !ok || len(bindings) == 0 {
		t.Error("Expected bindings in input")
	}

	bindingData, ok := bindings["test-binding"].(map[string]interface{})
	if !ok {
		t.Error("Expected binding data")
	}

	if bindingData["cluster_admin"] != true {
		t.Error("Expected cluster_admin to be true")
	}
}
