package graphql

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
	"github.com/jamesolaitan/accessgraph/internal/policy"
	"github.com/jamesolaitan/accessgraph/internal/store"
)

// --- Mock DataStore ---

type mockStore struct {
	snapshots  []store.Snapshot
	nodes      map[string]map[string]*ingest.Node // snapshotID -> nodeID -> Node
	edges      map[string][]ingest.Edge           // snapshotID -> edges
	principals map[string][]ingest.Node           // snapshotID -> matching nodes
	graph      *graph.Graph
}

func newMockStore() *mockStore {
	return &mockStore{
		nodes:      make(map[string]map[string]*ingest.Node),
		edges:      make(map[string][]ingest.Edge),
		principals: make(map[string][]ingest.Node),
	}
}

func (m *mockStore) ListSnapshots(_ context.Context) ([]store.Snapshot, error) {
	return m.snapshots, nil
}

func (m *mockStore) LoadSnapshot(_ context.Context, id string) (*graph.Graph, error) {
	if m.graph != nil {
		return m.graph, nil
	}
	return nil, fmt.Errorf("snapshot not found: %s", id)
}

func (m *mockStore) SearchPrincipals(_ context.Context, snapshotID, query string, limit int) ([]ingest.Node, error) {
	nodes := m.principals[snapshotID]
	var results []ingest.Node
	for _, n := range nodes {
		if limit > 0 && len(results) >= limit {
			break
		}
		results = append(results, n)
	}
	return results, nil
}

func (m *mockStore) GetNode(_ context.Context, snapshotID, nodeID string) (*ingest.Node, error) {
	if nodes, ok := m.nodes[snapshotID]; ok {
		if node, ok := nodes[nodeID]; ok {
			return node, nil
		}
	}
	return nil, fmt.Errorf("node not found: %s", nodeID)
}

func (m *mockStore) GetEdges(_ context.Context, snapshotID string) ([]ingest.Edge, error) {
	return m.edges[snapshotID], nil
}

func (m *mockStore) CountNodes(_ context.Context, snapshotID string) (int, error) {
	return len(m.nodes[snapshotID]), nil
}

func (m *mockStore) CountEdges(_ context.Context, snapshotID string) (int, error) {
	return len(m.edges[snapshotID]), nil
}

// --- Mock PolicyEvaluator ---

type mockEvaluator struct {
	findings []policy.Finding
	err      error
}

func (m *mockEvaluator) Evaluate(_ context.Context, _ map[string]interface{}) ([]policy.Finding, error) {
	return m.findings, m.err
}

// --- Helper ---

func newTestResolver(s DataStore, e PolicyEvaluator) *Resolver {
	return &Resolver{
		store:     s,
		evaluator: e,
		cache:     newGraphCache(),
	}
}

func defaultSnapshot() store.Snapshot {
	return store.Snapshot{
		ID:        "snap-1",
		Label:     "test-snapshot",
		CreatedAt: time.Date(2025, 10, 1, 12, 0, 0, 0, time.UTC),
	}
}

// --- Tests ---

func TestSearchPrincipals_ReturnsResults(t *testing.T) {
	ms := newMockStore()
	ms.snapshots = []store.Snapshot{defaultSnapshot()}
	ms.principals["snap-1"] = []ingest.Node{
		{ID: "arn:aws:iam::123456789012:role/DevRole", Kind: ingest.KindPrincipal, Labels: []string{"aws"}},
		{ID: "arn:aws:iam::123456789012:role/AdminRole", Kind: ingest.KindPrincipal, Labels: []string{"aws"}},
	}

	r := newTestResolver(ms, &mockEvaluator{})
	qr := &queryResolver{r}

	results, err := qr.SearchPrincipals(context.Background(), "Role", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	if results[0].ID != "arn:aws:iam::123456789012:role/DevRole" {
		t.Errorf("expected DevRole, got %s", results[0].ID)
	}
}

func TestSearchPrincipals_EmptySnapshot(t *testing.T) {
	ms := newMockStore()
	ms.snapshots = []store.Snapshot{} // no snapshots

	r := newTestResolver(ms, &mockEvaluator{})
	qr := &queryResolver{r}

	results, err := qr.SearchPrincipals(context.Background(), "anything", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty snapshots, got %d", len(results))
	}
}

func TestShortestPath_PathFound(t *testing.T) {
	g := graph.New()
	g.AddNode(ingest.Node{ID: "role1", Kind: ingest.KindPrincipal, Labels: []string{"aws"}})
	g.AddNode(ingest.Node{ID: "policy1", Kind: ingest.KindPolicy, Labels: []string{"aws"}})
	g.AddNode(ingest.Node{ID: "resource1", Kind: ingest.KindResource, Labels: []string{"aws"}})
	g.AddEdge(ingest.Edge{Src: "role1", Dst: "policy1", Kind: ingest.EdgeAttachedPolicy})
	g.AddEdge(ingest.Edge{Src: "policy1", Dst: "resource1", Kind: ingest.EdgeAppliesTo})

	ms := newMockStore()
	ms.snapshots = []store.Snapshot{defaultSnapshot()}
	ms.graph = g

	r := newTestResolver(ms, &mockEvaluator{})
	qr := &queryResolver{r}

	path, err := qr.ShortestPath(context.Background(), "role1", "resource1", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(path.Nodes) != 3 {
		t.Errorf("expected 3 nodes in path, got %d", len(path.Nodes))
	}

	if len(path.Edges) != 2 {
		t.Errorf("expected 2 edges in path, got %d", len(path.Edges))
	}
}

func TestFindings_ReturnsViolations(t *testing.T) {
	g := graph.New()
	g.AddNode(ingest.Node{ID: "role1", Kind: ingest.KindPrincipal, Labels: []string{"aws"}})

	ms := newMockStore()
	ms.snapshots = []store.Snapshot{defaultSnapshot()}
	ms.graph = g

	me := &mockEvaluator{
		findings: []policy.Finding{
			{RuleID: "overly-permissive", Severity: "HIGH", EntityRef: "role1", Reason: "wildcard access", Remediation: "restrict actions"},
			{RuleID: "cross-account", Severity: "MEDIUM", EntityRef: "role1", Reason: "cross-account trust", Remediation: "review trust policy"},
		},
	}

	r := newTestResolver(ms, me)
	qr := &queryResolver{r}

	findings, err := qr.Findings(context.Background(), "snap-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(findings) != 2 {
		t.Errorf("expected 2 findings, got %d", len(findings))
	}

	if findings[0].RuleID != "overly-permissive" {
		t.Errorf("expected rule 'overly-permissive', got %s", findings[0].RuleID)
	}

	if findings[0].Severity != "HIGH" {
		t.Errorf("expected severity 'HIGH', got %s", findings[0].Severity)
	}
}

func TestLoadGraph_CacheHit(t *testing.T) {
	g := graph.New()
	g.AddNode(ingest.Node{ID: "node1", Kind: ingest.KindPrincipal, Labels: []string{"aws"}})

	callCount := 0
	ms := &countingMockStore{
		mockStore: newMockStore(),
		graph:     g,
		loadCount: &callCount,
	}
	ms.mockStore.snapshots = []store.Snapshot{defaultSnapshot()}

	r := newTestResolver(ms, &mockEvaluator{})

	ctx := context.Background()

	// First call should hit the store
	g1, err := r.loadGraph(ctx, "snap-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 store call, got %d", callCount)
	}

	// Second call should use the cache
	g2, err := r.loadGraph(ctx, "snap-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected still 1 store call (cache hit), got %d", callCount)
	}

	// Should return the same graph instance
	if g1 != g2 {
		t.Error("expected cache to return the same graph instance")
	}
}

// countingMockStore wraps mockStore and counts LoadSnapshot calls.
type countingMockStore struct {
	*mockStore
	graph     *graph.Graph
	loadCount *int
}

func (c *countingMockStore) LoadSnapshot(_ context.Context, id string) (*graph.Graph, error) {
	*c.loadCount++
	if c.graph != nil {
		return c.graph, nil
	}
	return nil, fmt.Errorf("snapshot not found: %s", id)
}
