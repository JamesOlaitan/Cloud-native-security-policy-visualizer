package graphql

import (
	"context"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/jamesolaitan/accessgraph/internal/config"
	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
	"github.com/jamesolaitan/accessgraph/internal/policy"
	"github.com/jamesolaitan/accessgraph/internal/reco"
	"github.com/jamesolaitan/accessgraph/internal/store"
)

// DataStore defines the storage operations the resolver depends on.
// Defined at the consumer side so the resolver is testable without a real database.
type DataStore interface {
	ListSnapshots(ctx context.Context) ([]store.Snapshot, error)
	LoadSnapshot(ctx context.Context, id string) (*graph.Graph, error)
	SearchPrincipals(ctx context.Context, snapshotID, query string, limit int) ([]ingest.Node, error)
	GetNode(ctx context.Context, snapshotID, nodeID string) (*ingest.Node, error)
	GetEdges(ctx context.Context, snapshotID string) ([]ingest.Edge, error)
	CountNodes(ctx context.Context, snapshotID string) (int, error)
	CountEdges(ctx context.Context, snapshotID string) (int, error)
}

// PolicyEvaluator defines the policy evaluation operations the resolver depends on.
type PolicyEvaluator interface {
	Evaluate(ctx context.Context, input map[string]interface{}) ([]policy.Finding, error)
}

// graphCache provides a bounded in-memory LRU cache for loaded graphs keyed by
// snapshot ID. This avoids reloading from the database on every query while
// preventing unbounded memory growth.
const maxCachedGraphs = 16

type graphCache struct {
	cache *lru.Cache[string, *graph.Graph]
}

func newGraphCache() *graphCache {
	c, _ := lru.New[string, *graph.Graph](maxCachedGraphs)
	return &graphCache{cache: c}
}

func (c *graphCache) get(snapshotID string) (*graph.Graph, bool) {
	return c.cache.Get(snapshotID)
}

func (c *graphCache) set(snapshotID string, g *graph.Graph) {
	c.cache.Add(snapshotID, g)
}

// Resolver is the root GraphQL resolver
type Resolver struct {
	store     DataStore
	evaluator PolicyEvaluator
	config    *config.Config
	cache     *graphCache
}

// NewResolver creates a new resolver
func NewResolver(store DataStore, cfg *config.Config) *Resolver {
	return &Resolver{
		store:     store,
		evaluator: policy.NewClient(cfg.OPAUrl),
		config:    cfg,
		cache:     newGraphCache(),
	}
}

// Default values for query parameters
const (
	DefaultMaxHops      = 8
	DefaultSearchLimit  = 10
	DefaultRecommendCap = 20
)

// loadGraph loads a graph from cache or store, caching the result.
func (r *Resolver) loadGraph(ctx context.Context, snapshotID string) (*graph.Graph, error) {
	if g, ok := r.cache.get(snapshotID); ok {
		return g, nil
	}

	g, err := r.store.LoadSnapshot(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	r.cache.set(snapshotID, g)
	return g, nil
}

// getLatestSnapshotID retrieves the most recent snapshot ID from the store.
// Returns an error if no snapshots exist.
func (r *Resolver) getLatestSnapshotID(ctx context.Context) (string, error) {
	snapshots, err := r.store.ListSnapshots(ctx)
	if err != nil {
		return "", err
	}
	if len(snapshots) == 0 {
		return "", fmt.Errorf("no snapshots found")
	}
	return snapshots[0].ID, nil
}

// Query returns the query resolver
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

type queryResolver struct{ *Resolver }

// SearchPrincipals searches for principal nodes
func (r *queryResolver) SearchPrincipals(ctx context.Context, query string, limit *int) ([]*Node, error) {
	snapshotID, err := r.getLatestSnapshotID(ctx)
	if err != nil {
		// Return empty list instead of error for no snapshots
		return []*Node{}, nil
	}

	l := DefaultSearchLimit
	if limit != nil && *limit > 0 {
		l = *limit
	}

	nodes, err := r.store.SearchPrincipals(ctx, snapshotID, query, l)
	if err != nil {
		return nil, err
	}

	result := make([]*Node, len(nodes))
	for i, node := range nodes {
		result[i] = nodeToGraphQL(node)
	}

	return result, nil
}

// Node retrieves a single node with its neighbors
func (r *queryResolver) Node(ctx context.Context, id string) (*Node, error) {
	snapshotID, err := r.getLatestSnapshotID(ctx)
	if err != nil {
		return nil, err
	}

	node, err := r.store.GetNode(ctx, snapshotID, id)
	if err != nil {
		return nil, err
	}

	result := nodeToGraphQL(*node)

	// Also fetch neighbors for this node
	g, err := r.loadGraph(ctx, snapshotID)
	if err != nil {
		return result, nil // Return node without neighbors on graph load error
	}

	neighbors, edgeKinds, err := g.GetNeighbors(id, nil)
	if err != nil {
		return result, nil // Return node without neighbors on error
	}

	result.Neighbors = make([]*Neighbor, len(neighbors))
	for i, neighbor := range neighbors {
		result.Neighbors[i] = &Neighbor{
			ID:       neighbor.ID,
			Kind:     string(neighbor.Kind),
			Labels:   neighbor.Labels,
			EdgeKind: edgeKinds[i],
		}
	}

	return result, nil
}

// ShortestPath finds the shortest path between two nodes
func (r *queryResolver) ShortestPath(ctx context.Context, from string, to string, maxHops *int) (*Path, error) {
	snapshotID, err := r.getLatestSnapshotID(ctx)
	if err != nil {
		return nil, err
	}

	g, err := r.loadGraph(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	hops := DefaultMaxHops
	if maxHops != nil && *maxHops > 0 {
		hops = *maxHops
	}

	nodes, edges, err := g.ShortestPath(from, to, hops)
	if err != nil {
		return nil, err
	}

	pathNodes := make([]*Node, len(nodes))
	for i, node := range nodes {
		pathNodes[i] = nodeToGraphQL(node)
	}

	pathEdges := make([]*Edge, len(edges))
	for i, edge := range edges {
		pathEdges[i] = &Edge{
			From: edge.Src,
			To:   edge.Dst,
			Kind: edge.Kind,
		}
	}

	return &Path{
		Nodes: pathNodes,
		Edges: pathEdges,
	}, nil
}

// Findings returns policy violations for a snapshot
func (r *queryResolver) Findings(ctx context.Context, snapshotID string) ([]*Finding, error) {
	g, err := r.loadGraph(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	input := policy.BuildInput(g)

	violations, err := r.evaluator.Evaluate(ctx, input)
	if err != nil {
		return nil, err
	}

	findings := make([]*Finding, len(violations))
	for i, v := range violations {
		findings[i] = &Finding{
			ID:          fmt.Sprintf("%s-%d", v.RuleID, i),
			RuleID:      v.RuleID,
			Severity:    v.Severity,
			EntityRef:   v.EntityRef,
			Reason:      v.Reason,
			Remediation: v.Remediation,
		}
	}

	return findings, nil
}

// Snapshots returns all snapshots
func (r *queryResolver) Snapshots(ctx context.Context) ([]*Snapshot, error) {
	snapshots, err := r.store.ListSnapshots(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*Snapshot, len(snapshots))
	for i, snap := range snapshots {
		nodeCount, err := r.store.CountNodes(ctx, snap.ID)
		if err != nil {
			return nil, fmt.Errorf("counting nodes for snapshot %s: %w", snap.ID, err)
		}

		edgeCount, err := r.store.CountEdges(ctx, snap.ID)
		if err != nil {
			return nil, fmt.Errorf("counting edges for snapshot %s: %w", snap.ID, err)
		}

		label := snap.Label
		result[i] = &Snapshot{
			ID:        snap.ID,
			CreatedAt: snap.CreatedAt.Format("2006-01-02 15:04:05"),
			Label:     &label,
			NodeCount: nodeCount,
			EdgeCount: edgeCount,
		}
	}

	return result, nil
}

// SnapshotDiff computes the diff between two snapshots
func (r *queryResolver) SnapshotDiff(ctx context.Context, a string, b string) (*SnapshotDiff, error) {
	edgesA, err := r.store.GetEdges(ctx, a)
	if err != nil {
		return nil, err
	}

	edgesB, err := r.store.GetEdges(ctx, b)
	if err != nil {
		return nil, err
	}

	// Create edge maps for comparison
	edgeMapA := make(map[string]ingest.Edge)
	edgeMapB := make(map[string]ingest.Edge)

	for _, edge := range edgesA {
		edgeMapA[edge.Key()] = edge
	}

	for _, edge := range edgesB {
		edgeMapB[edge.Key()] = edge
	}

	// Find added and removed edges
	addedEdges := []*Edge{}
	removedEdges := []*Edge{}

	for key, edge := range edgeMapB {
		if _, exists := edgeMapA[key]; !exists {
			addedEdges = append(addedEdges, &Edge{
				From: edge.Src,
				To:   edge.Dst,
				Kind: edge.Kind,
			})
		}
	}

	for key, edge := range edgeMapA {
		if _, exists := edgeMapB[key]; !exists {
			removedEdges = append(removedEdges, &Edge{
				From: edge.Src,
				To:   edge.Dst,
				Kind: edge.Kind,
			})
		}
	}

	return &SnapshotDiff{
		AddedEdges:   addedEdges,
		RemovedEdges: removedEdges,
		Summary: &DiffSummary{
			Added:   len(addedEdges),
			Removed: len(removedEdges),
			Changed: 0, // Simplified for MVP
		},
	}, nil
}

// Helper functions

func nodeToGraphQL(node ingest.Node) *Node {
	props := make([]*Kv, 0, len(node.Props))
	for k, v := range node.Props {
		props = append(props, &Kv{Key: k, Value: v})
	}

	return &Node{
		ID:     node.ID,
		Kind:   string(node.Kind),
		Labels: node.Labels,
		Props:  props,
	}
}

// ============ Phase 2 Resolvers ============

// AttackPath finds an attack path from a principal to a resource
func (r *queryResolver) AttackPath(ctx context.Context, from string, to *string, tags []string, maxHops *int) (*Path, error) {
	snapshotID, err := r.getLatestSnapshotID(ctx)
	if err != nil {
		return nil, err
	}

	g, err := r.loadGraph(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	hops := DefaultMaxHops
	if maxHops != nil && *maxHops > 0 {
		hops = *maxHops
	}

	toID := ""
	if to != nil {
		toID = *to
	}

	result, err := g.FindAttackPath(from, toID, tags, hops)
	if err != nil {
		return nil, err
	}

	if !result.Found {
		return nil, fmt.Errorf("no attack path found")
	}

	pathNodes := make([]*Node, len(result.Nodes))
	for i, node := range result.Nodes {
		pathNodes[i] = nodeToGraphQL(node)
	}

	pathEdges := make([]*Edge, len(result.Edges))
	for i, edge := range result.Edges {
		pathEdges[i] = &Edge{
			From: edge.Src,
			To:   edge.Dst,
			Kind: edge.Kind,
		}
	}

	return &Path{
		Nodes: pathNodes,
		Edges: pathEdges,
	}, nil
}

// Recommend generates least-privilege recommendations for a policy
func (r *queryResolver) Recommend(ctx context.Context, snapshotID string, policyID string, target *string, tags []string, cap *int) (*Recommendation, error) {
	g, err := r.loadGraph(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	recommender := reco.New(g)

	capVal := DefaultRecommendCap
	if cap != nil && *cap > 0 {
		capVal = *cap
	}

	targetID := ""
	if target != nil {
		targetID = *target
	}

	rec, err := recommender.Recommend(policyID, targetID, tags, capVal)
	if err != nil {
		return nil, err
	}

	return &Recommendation{
		PolicyID:           rec.PolicyID,
		SuggestedActions:   rec.SuggestedActions,
		SuggestedResources: rec.SuggestedResources,
		PatchJSON:          rec.PatchJSON,
		Rationale:          rec.Rationale,
	}, nil
}

// ExportCypher exports the graph to Neo4j Cypher format
func (r *queryResolver) ExportCypher(ctx context.Context, snapshotID string) (*Export, error) {
	g, err := r.loadGraph(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	cypher, err := g.ExportCypher()
	if err != nil {
		return nil, err
	}

	return &Export{
		Filename: fmt.Sprintf("accessgraph-%s.cypher", snapshotID),
		Content:  cypher,
	}, nil
}

// ExportMarkdownAttackPath exports an attack path as Markdown
func (r *queryResolver) ExportMarkdownAttackPath(ctx context.Context, from string, to string) (*Export, error) {
	snapshotID, err := r.getLatestSnapshotID(ctx)
	if err != nil {
		return nil, err
	}

	g, err := r.loadGraph(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	// Find the attack path
	result, err := g.FindAttackPath(from, to, nil, DefaultMaxHops)
	if err != nil {
		return nil, err
	}

	if !result.Found {
		return nil, fmt.Errorf("no attack path found")
	}

	// Export to Markdown
	markdown, err := graph.ExportMarkdownAttackPath(from, to, result.Nodes, result.Edges)
	if err != nil {
		return nil, err
	}

	return &Export{
		Filename: fmt.Sprintf("attack-path-%s.md", snapshotID),
		Content:  markdown,
	}, nil
}

// ExportSarifAttackPath exports an attack path as SARIF
func (r *queryResolver) ExportSarifAttackPath(ctx context.Context, from string, to string) (*Export, error) {
	snapshotID, err := r.getLatestSnapshotID(ctx)
	if err != nil {
		return nil, err
	}

	g, err := r.loadGraph(ctx, snapshotID)
	if err != nil {
		return nil, err
	}

	// Find the attack path
	result, err := g.FindAttackPath(from, to, nil, DefaultMaxHops)
	if err != nil {
		return nil, err
	}

	if !result.Found {
		return nil, fmt.Errorf("no attack path found")
	}

	// Export to SARIF
	sarif, err := graph.ExportSARIFAttackPath(from, to, result.Nodes, result.Edges)
	if err != nil {
		return nil, err
	}

	return &Export{
		Filename: fmt.Sprintf("attack-path-%s.sarif", snapshotID),
		Content:  sarif,
	}, nil
}
