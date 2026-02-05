package graph

import (
	"fmt"

	"github.com/jamesolaitan/accessgraph/internal/ingest"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/traverse"
)

// Default values for graph operations
const (
	DefaultMaxHops  = 8
	DefaultBFSDepth = 3
)

// Graph wraps a directed multigraph with node/edge management
type Graph struct {
	g         *simple.DirectedGraph
	nodes     map[string]*graphNode
	edges     []ingest.Edge
	nodesByID map[int64]string
	// edgeIndex provides O(1) edge lookup by source and destination node IDs,
	// avoiding O(E) scans when resolving neighbors or shortest paths.
	edgeIndex map[string]map[string][]ingest.Edge
}

type graphNode struct {
	id   int64
	data ingest.Node
}

func (n *graphNode) ID() int64 {
	return n.id
}

// New creates a new graph
func New() *Graph {
	return &Graph{
		g:         simple.NewDirectedGraph(),
		nodes:     make(map[string]*graphNode),
		edges:     []ingest.Edge{},
		nodesByID: make(map[int64]string),
		edgeIndex: make(map[string]map[string][]ingest.Edge),
	}
}

// AddNode adds a node to the graph
func (g *Graph) AddNode(node ingest.Node) {
	if _, exists := g.nodes[node.ID]; exists {
		return
	}

	id := int64(len(g.nodes) + 1)
	gn := &graphNode{
		id:   id,
		data: node,
	}

	g.nodes[node.ID] = gn
	g.nodesByID[id] = node.ID
	g.g.AddNode(gn)
}

// AddEdge adds an edge to the graph
func (g *Graph) AddEdge(edge ingest.Edge) error {
	src, ok := g.nodes[edge.Src]
	if !ok {
		return fmt.Errorf("source node not found: %s", edge.Src)
	}

	dst, ok := g.nodes[edge.Dst]
	if !ok {
		return fmt.Errorf("destination node not found: %s", edge.Dst)
	}

	g.g.SetEdge(g.g.NewEdge(src, dst))
	g.edges = append(g.edges, edge)

	// Update edge index for O(1) lookups
	if g.edgeIndex[edge.Src] == nil {
		g.edgeIndex[edge.Src] = make(map[string][]ingest.Edge)
	}
	g.edgeIndex[edge.Src][edge.Dst] = append(g.edgeIndex[edge.Src][edge.Dst], edge)

	return nil
}

// GetNode retrieves a node by ID
func (g *Graph) GetNode(id string) (ingest.Node, bool) {
	node, ok := g.nodes[id]
	if !ok {
		return ingest.Node{}, false
	}
	return node.data, true
}

// GetNodes returns all nodes
func (g *Graph) GetNodes() []ingest.Node {
	nodes := make([]ingest.Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node.data)
	}
	return nodes
}

// GetEdges returns all edges
func (g *Graph) GetEdges() []ingest.Edge {
	return g.edges
}

// lookupEdgeKind returns the kind of the first edge between src and dst using
// the edge index. Falls back to empty string if no edge is found.
func (g *Graph) lookupEdgeKind(srcID, dstID string) string {
	if dstMap, ok := g.edgeIndex[srcID]; ok {
		if edges, ok := dstMap[dstID]; ok && len(edges) > 0 {
			return edges[0].Kind
		}
	}
	return ""
}

// lookupEdge returns the first edge between src and dst using the edge index.
func (g *Graph) lookupEdge(srcID, dstID string) (ingest.Edge, bool) {
	if dstMap, ok := g.edgeIndex[srcID]; ok {
		if edges, ok := dstMap[dstID]; ok && len(edges) > 0 {
			return edges[0], true
		}
	}
	return ingest.Edge{}, false
}

// GetNeighbors returns neighbors of a node filtered by kind
func (g *Graph) GetNeighbors(id string, kinds []ingest.Kind) ([]ingest.Node, []string, error) {
	node, ok := g.nodes[id]
	if !ok {
		return nil, nil, fmt.Errorf("node not found: %s", id)
	}

	neighbors := []ingest.Node{}
	edgeKinds := []string{}
	kindsMap := make(map[ingest.Kind]bool)
	for _, k := range kinds {
		kindsMap[k] = true
	}

	// Get outgoing edges
	to := g.g.From(node.ID())
	for to.Next() {
		neighborID := to.Node().ID()
		neighborNodeID := g.nodesByID[neighborID]
		neighborData := g.nodes[neighborNodeID].data

		if len(kinds) == 0 || kindsMap[neighborData.Kind] {
			edgeKind := g.lookupEdgeKind(id, neighborNodeID)
			neighbors = append(neighbors, neighborData)
			edgeKinds = append(edgeKinds, edgeKind)
		}
	}

	// Get incoming edges
	from := g.g.To(node.ID())
	for from.Next() {
		neighborID := from.Node().ID()
		neighborNodeID := g.nodesByID[neighborID]
		neighborData := g.nodes[neighborNodeID].data

		if len(kinds) == 0 || kindsMap[neighborData.Kind] {
			edgeKind := g.lookupEdgeKind(neighborNodeID, id)
			neighbors = append(neighbors, neighborData)
			edgeKinds = append(edgeKinds, edgeKind)
		}
	}

	return neighbors, edgeKinds, nil
}

// ShortestPath finds the shortest path between two nodes using BFS
func (g *Graph) ShortestPath(fromID, toID string, maxHops int) ([]ingest.Node, []ingest.Edge, error) {
	srcNode, ok := g.nodes[fromID]
	if !ok {
		return nil, nil, fmt.Errorf("source node not found: %s", fromID)
	}

	dstNode, ok := g.nodes[toID]
	if !ok {
		return nil, nil, fmt.Errorf("destination node not found: %s", toID)
	}

	if maxHops <= 0 {
		maxHops = DefaultMaxHops
	}

	// Use BFS to find shortest path
	shortest := path.DijkstraFrom(srcNode, g.g)
	nodePath, _ := shortest.To(dstNode.ID())

	if len(nodePath) == 0 {
		return nil, nil, fmt.Errorf("no path found")
	}

	if len(nodePath) > maxHops+1 {
		return nil, nil, fmt.Errorf("path exceeds max hops")
	}

	// Convert to node/edge lists
	nodes := []ingest.Node{}
	edges := []ingest.Edge{}

	for i, gn := range nodePath {
		nodeID := g.nodesByID[gn.ID()]
		nodes = append(nodes, g.nodes[nodeID].data)

		if i < len(nodePath)-1 {
			nextID := g.nodesByID[nodePath[i+1].ID()]
			if edge, found := g.lookupEdge(nodeID, nextID); found {
				edges = append(edges, edge)
			}
		}
	}

	return nodes, edges, nil
}

// BFS performs a breadth-first search starting from a node
func (g *Graph) BFS(startID string, maxDepth int) ([]ingest.Node, error) {
	startNode, ok := g.nodes[startID]
	if !ok {
		return nil, fmt.Errorf("start node not found: %s", startID)
	}

	if maxDepth <= 0 {
		maxDepth = DefaultBFSDepth
	}

	visited := make(map[int64]bool)
	result := []ingest.Node{}

	bfs := traverse.BreadthFirst{
		Traverse: func(e graph.Edge) bool {
			// Use the Walk callback's depth parameter instead of result length
			// to correctly gate traversal. The Traverse function controls which
			// edges are followed, but depth gating happens in Walk below.
			return true
		},
	}

	bfs.Walk(g.g, startNode, func(n graph.Node, depth int) bool {
		if depth > maxDepth {
			return true // stop exploring beyond maxDepth
		}
		if !visited[n.ID()] {
			visited[n.ID()] = true
			nodeID := g.nodesByID[n.ID()]
			result = append(result, g.nodes[nodeID].data)
		}
		return false
	})

	return result, nil
}
