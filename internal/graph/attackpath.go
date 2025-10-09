package graph

import (
	"fmt"
	"sort"

	"github.com/jamesolaitan/accessgraph/internal/ingest"
)

// AttackPathResult represents the result of an attack path query
type AttackPathResult struct {
	Nodes []ingest.Node
	Edges []ingest.Edge
	Found bool
}

// FindAttackPath finds the shortest path from a principal to a target resource
// If toID is empty and tags includes "sensitive", it finds the nearest sensitive resource
func (g *Graph) FindAttackPath(fromID, toID string, tags []string, maxHops int) (*AttackPathResult, error) {
	if maxHops <= 0 {
		maxHops = 8
	}

	// Validate source node exists
	if _, ok := g.nodes[fromID]; !ok {
		return nil, fmt.Errorf("source node not found: %s", fromID)
	}

	// If toID is provided, use direct shortest path
	if toID != "" {
		// Check if target exists
		if _, ok := g.nodes[toID]; !ok {
			return nil, fmt.Errorf("destination node not found: %s", toID)
		}

		nodes, edges, err := g.ShortestPath(fromID, toID, maxHops)
		if err != nil {
			return &AttackPathResult{Found: false}, nil
		}
		return &AttackPathResult{
			Nodes: nodes,
			Edges: edges,
			Found: true,
		}, nil
	}

	// If toID is empty and tags includes "sensitive", find nearest sensitive resource
	if containsTag(tags, "sensitive") {
		return g.findNearestSensitiveResource(fromID, maxHops)
	}

	return nil, fmt.Errorf("target ID or 'sensitive' tag required")
}

// findNearestSensitiveResource finds the shortest path to any sensitive resource
func (g *Graph) findNearestSensitiveResource(fromID string, maxHops int) (*AttackPathResult, error) {
	// Find all sensitive resources
	sensitiveResources := g.findSensitiveResources()
	if len(sensitiveResources) == 0 {
		return &AttackPathResult{Found: false}, nil
	}

	// Sort for determinism
	sort.Strings(sensitiveResources)

	// Try to find shortest path to any sensitive resource
	var shortestPath *AttackPathResult
	shortestLength := maxHops + 1

	for _, targetID := range sensitiveResources {
		nodes, edges, err := g.ShortestPath(fromID, targetID, maxHops)
		if err != nil {
			continue
		}

		// Check if this is the shortest path found so far
		if len(nodes) < shortestLength {
			shortestLength = len(nodes)
			shortestPath = &AttackPathResult{
				Nodes: nodes,
				Edges: edges,
				Found: true,
			}
		}
	}

	if shortestPath == nil {
		return &AttackPathResult{Found: false}, nil
	}

	return shortestPath, nil
}

// findSensitiveResources returns IDs of all nodes marked as sensitive (sorted)
func (g *Graph) findSensitiveResources() []string {
	var sensitive []string
	for id, node := range g.nodes {
		if val, ok := node.data.Props["sensitive"]; ok && val == "true" {
			sensitive = append(sensitive, id)
		}
	}
	sort.Strings(sensitive)
	return sensitive
}

// containsTag checks if a tag is in the list
func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// GetEdgeDetails returns detailed information about an edge
func (g *Graph) GetEdgeDetails(srcID, dstID string) (*ingest.Edge, error) {
	for _, e := range g.edges {
		if e.Src == srcID && e.Dst == dstID {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("edge not found from %s to %s", srcID, dstID)
}

// MarkSensitive marks a node as sensitive
func (g *Graph) MarkSensitive(nodeID string) error {
	node, ok := g.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	if node.data.Props == nil {
		node.data.Props = make(map[string]string)
	}
	node.data.Props["sensitive"] = "true"

	return nil
}
