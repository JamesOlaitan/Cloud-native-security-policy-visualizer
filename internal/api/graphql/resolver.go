package graphql

import (
	"context"
	"fmt"
	"strings"

	"github.com/jamesolaitan/accessgraph/internal/config"
	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
	"github.com/jamesolaitan/accessgraph/internal/policy"
	"github.com/jamesolaitan/accessgraph/internal/reco"
	"github.com/jamesolaitan/accessgraph/internal/store"
)

// Resolver is the root GraphQL resolver
type Resolver struct {
	store     *store.Store
	opaClient *policy.Client
	config    *config.Config
}

// NewResolver creates a new resolver
func NewResolver(store *store.Store, cfg *config.Config) *Resolver {
	return &Resolver{
		store:     store,
		opaClient: policy.NewClient(cfg.OPAUrl),
		config:    cfg,
	}
}

// Query returns the query resolver
func (r *Resolver) Query() QueryResolver {
	return &queryResolver{r}
}

// Node returns the node resolver
func (r *Resolver) Node() NodeResolver {
	return &nodeResolver{r}
}

type queryResolver struct{ *Resolver }
type nodeResolver struct{ *Resolver }

// SearchPrincipals searches for principal nodes
func (r *queryResolver) SearchPrincipals(ctx context.Context, query string, limit *int) ([]*Node, error) {
	// Get the most recent snapshot
	snapshots, err := r.store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return []*Node{}, nil
	}

	snapshotID := snapshots[0].ID

	l := 10
	if limit != nil && *limit > 0 {
		l = *limit
	}

	nodes, err := r.store.SearchPrincipals(snapshotID, query, l)
	if err != nil {
		return nil, err
	}

	result := make([]*Node, len(nodes))
	for i, node := range nodes {
		result[i] = nodeToGraphQL(node)
	}

	return result, nil
}

// Node retrieves a single node
func (r *queryResolver) Node(ctx context.Context, id string) (*Node, error) {
	// Get the most recent snapshot
	snapshots, err := r.store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}

	snapshotID := snapshots[0].ID

	node, err := r.store.GetNode(snapshotID, id)
	if err != nil {
		return nil, err
	}

	return nodeToGraphQL(*node), nil
}

// ShortestPath finds the shortest path between two nodes
func (r *queryResolver) ShortestPath(ctx context.Context, from string, to string, maxHops *int) (*Path, error) {
	// Get the most recent snapshot
	snapshots, err := r.store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}

	snapshotID := snapshots[0].ID

	g, err := r.store.LoadSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	hops := 8
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
	g, err := r.store.LoadSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	input := policy.BuildInput(g)

	violations, err := r.opaClient.Evaluate(input)
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
	snapshots, err := r.store.ListSnapshots()
	if err != nil {
		return nil, err
	}

	result := make([]*Snapshot, len(snapshots))
	for i, snap := range snapshots {
		nodeCount, _ := r.store.CountNodes(snap.ID)
		edgeCount, _ := r.store.CountEdges(snap.ID)

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
	edgesA, err := r.store.GetEdges(a)
	if err != nil {
		return nil, err
	}

	edgesB, err := r.store.GetEdges(b)
	if err != nil {
		return nil, err
	}

	// Create edge maps for comparison
	edgeMapA := make(map[string]ingest.Edge)
	edgeMapB := make(map[string]ingest.Edge)

	for _, edge := range edgesA {
		key := edgeKey(edge)
		edgeMapA[key] = edge
	}

	for _, edge := range edgesB {
		key := edgeKey(edge)
		edgeMapB[key] = edge
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

// Neighbors resolves neighbors for a node
func (r *nodeResolver) Neighbors(ctx context.Context, obj *Node, kinds []*string) ([]*Neighbor, error) {
	// Get the most recent snapshot
	snapshots, err := r.store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return []*Neighbor{}, nil
	}

	snapshotID := snapshots[0].ID

	g, err := r.store.LoadSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	kindFilter := []ingest.Kind{}
	for _, k := range kinds {
		if k != nil {
			kindFilter = append(kindFilter, ingest.Kind(*k))
		}
	}

	neighbors, edgeKinds, err := g.GetNeighbors(obj.ID, kindFilter)
	if err != nil {
		return nil, err
	}

	result := make([]*Neighbor, len(neighbors))
	for i, neighbor := range neighbors {
		result[i] = &Neighbor{
			ID:       neighbor.ID,
			Kind:     string(neighbor.Kind),
			Labels:   neighbor.Labels,
			EdgeKind: edgeKinds[i],
		}
	}

	return result, nil
}

// Helper functions

func nodeToGraphQL(node ingest.Node) *Node {
	props := make([]*KV, 0, len(node.Props))
	for k, v := range node.Props {
		props = append(props, &KV{Key: k, Value: v})
	}

	return &Node{
		ID:     node.ID,
		Kind:   string(node.Kind),
		Labels: node.Labels,
		Props:  props,
	}
}

func edgeKey(edge ingest.Edge) string {
	return strings.Join([]string{edge.Src, edge.Dst, edge.Kind}, "|")
}

// ============ Phase 2 Resolvers ============

// AttackPath finds an attack path from a principal to a resource
func (r *queryResolver) AttackPath(ctx context.Context, from string, to *string, tags []string, maxHops *int) (*Path, error) {
	// Get the most recent snapshot
	snapshots, err := r.store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}

	snapshotID := snapshots[0].ID

	g, err := r.store.LoadSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	hops := 8
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
	g, err := r.store.LoadSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	recommender := reco.New(g)

	capVal := 20
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
	g, err := r.store.LoadSnapshot(snapshotID)
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
	// Get the most recent snapshot
	snapshots, err := r.store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}

	snapshotID := snapshots[0].ID

	g, err := r.store.LoadSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	// Find the attack path
	result, err := g.FindAttackPath(from, to, nil, 8)
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
	// Get the most recent snapshot
	snapshots, err := r.store.ListSnapshots()
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}

	snapshotID := snapshots[0].ID

	g, err := r.store.LoadSnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	// Find the attack path
	result, err := g.FindAttackPath(from, to, nil, 8)
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
