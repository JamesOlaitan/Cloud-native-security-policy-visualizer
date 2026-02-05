package store

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jamesolaitan/accessgraph/internal/graph"
	"github.com/jamesolaitan/accessgraph/internal/ingest"
	_ "modernc.org/sqlite"
)

//go:embed models.sql
var schema string

// Store manages SQLite persistence
type Store struct {
	db *sql.DB
}

// Snapshot represents a saved graph snapshot
type Snapshot struct {
	ID        string
	CreatedAt time.Time
	Label     string
}

// New creates a new store
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Initialize schema
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("initializing schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection
func (s *Store) Close() error {
	return s.db.Close()
}

// SaveSnapshot saves a graph snapshot
func (s *Store) SaveSnapshot(ctx context.Context, id, label string, g *graph.Graph) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Insert snapshot
	createdAt := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.ExecContext(ctx,
		"INSERT INTO snapshots (id, created_at, label) VALUES (?, ?, ?)",
		id, createdAt, label,
	)
	if err != nil {
		return fmt.Errorf("inserting snapshot: %w", err)
	}

	// Insert nodes
	nodes := g.GetNodes()
	for _, node := range nodes {
		labelsJSON, err := json.Marshal(node.Labels)
		if err != nil {
			return fmt.Errorf("marshaling labels for node %s: %w", node.ID, err)
		}

		propsJSON, err := json.Marshal(node.Props)
		if err != nil {
			return fmt.Errorf("marshaling props for node %s: %w", node.ID, err)
		}

		_, err = tx.ExecContext(ctx,
			"INSERT INTO nodes (snapshot_id, id, kind, labels, props) VALUES (?, ?, ?, ?, ?)",
			id, node.ID, string(node.Kind), string(labelsJSON), string(propsJSON),
		)
		if err != nil {
			return fmt.Errorf("inserting node %s: %w", node.ID, err)
		}
	}

	// Insert edges
	edges := g.GetEdges()
	for _, edge := range edges {
		propsJSON, err := json.Marshal(edge.Props)
		if err != nil {
			return fmt.Errorf("marshaling props for edge %s->%s: %w", edge.Src, edge.Dst, err)
		}

		_, err = tx.ExecContext(ctx,
			"INSERT INTO edges (snapshot_id, src, dst, kind, props) VALUES (?, ?, ?, ?, ?)",
			id, edge.Src, edge.Dst, edge.Kind, string(propsJSON),
		)
		if err != nil {
			return fmt.Errorf("inserting edge: %w", err)
		}
	}

	return tx.Commit()
}

// LoadSnapshot loads a graph snapshot
func (s *Store) LoadSnapshot(ctx context.Context, id string) (*graph.Graph, error) {
	g := graph.New()

	// Load nodes (ordered for determinism)
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, kind, labels, props FROM nodes WHERE snapshot_id = ? ORDER BY id",
		id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var node ingest.Node
		var labelsJSON, propsJSON string
		var kind string

		if err := rows.Scan(&node.ID, &kind, &labelsJSON, &propsJSON); err != nil {
			return nil, err
		}

		node.Kind = ingest.Kind(kind)
		if err := json.Unmarshal([]byte(labelsJSON), &node.Labels); err != nil {
			return nil, fmt.Errorf("unmarshaling labels for node %s: %w", node.ID, err)
		}
		if err := json.Unmarshal([]byte(propsJSON), &node.Props); err != nil {
			return nil, fmt.Errorf("unmarshaling props for node %s: %w", node.ID, err)
		}

		g.AddNode(node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load edges (ordered for determinism)
	edgeRows, err := s.db.QueryContext(ctx,
		"SELECT src, dst, kind, props FROM edges WHERE snapshot_id = ? ORDER BY src, dst, kind",
		id,
	)
	if err != nil {
		return nil, err
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var edge ingest.Edge
		var propsJSON string

		if err := edgeRows.Scan(&edge.Src, &edge.Dst, &edge.Kind, &propsJSON); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(propsJSON), &edge.Props); err != nil {
			return nil, fmt.Errorf("unmarshaling edge props %s->%s: %w", edge.Src, edge.Dst, err)
		}

		if err := g.AddEdge(edge); err != nil {
			// Skip edges with missing nodes
			continue
		}
	}
	if err := edgeRows.Err(); err != nil {
		return nil, err
	}

	return g, nil
}

// ListSnapshots returns all snapshots
func (s *Store) ListSnapshots(ctx context.Context) ([]Snapshot, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, created_at, label FROM snapshots ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		var snap Snapshot
		var createdAtStr string
		var label sql.NullString

		if err := rows.Scan(&snap.ID, &createdAtStr, &label); err != nil {
			return nil, err
		}

		snap.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		if label.Valid {
			snap.Label = label.String
		}

		snapshots = append(snapshots, snap)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return snapshots, nil
}

// GetSnapshot retrieves a single snapshot by ID
func (s *Store) GetSnapshot(ctx context.Context, id string) (*Snapshot, error) {
	var snap Snapshot
	var createdAtStr string
	var label sql.NullString

	err := s.db.QueryRowContext(ctx,
		"SELECT id, created_at, label FROM snapshots WHERE id = ?",
		id,
	).Scan(&snap.ID, &createdAtStr, &label)

	if err != nil {
		return nil, err
	}

	snap.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	if label.Valid {
		snap.Label = label.String
	}

	return &snap, nil
}

// CountNodes returns the number of nodes in a snapshot
func (s *Store) CountNodes(ctx context.Context, snapshotID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM nodes WHERE snapshot_id = ?", snapshotID).Scan(&count)
	return count, err
}

// CountEdges returns the number of edges in a snapshot
func (s *Store) CountEdges(ctx context.Context, snapshotID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM edges WHERE snapshot_id = ?", snapshotID).Scan(&count)
	return count, err
}

// SearchPrincipals searches for principal nodes by query string
func (s *Store) SearchPrincipals(ctx context.Context, snapshotID, query string, limit int) ([]ingest.Node, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, kind, labels, props FROM nodes
		WHERE snapshot_id = ? AND kind = 'PRINCIPAL'
		AND (id LIKE ? OR labels LIKE ?)
		ORDER BY id
		LIMIT ?
	`, snapshotID, "%"+query+"%", "%"+query+"%", limit)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []ingest.Node
	for rows.Next() {
		var node ingest.Node
		var labelsJSON, propsJSON, kind string

		if err := rows.Scan(&node.ID, &kind, &labelsJSON, &propsJSON); err != nil {
			return nil, err
		}

		node.Kind = ingest.Kind(kind)
		if err := json.Unmarshal([]byte(labelsJSON), &node.Labels); err != nil {
			return nil, fmt.Errorf("unmarshaling labels: %w", err)
		}
		if err := json.Unmarshal([]byte(propsJSON), &node.Props); err != nil {
			return nil, fmt.Errorf("unmarshaling props: %w", err)
		}

		nodes = append(nodes, node)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nodes, nil
}

// GetNode retrieves a single node by ID
func (s *Store) GetNode(ctx context.Context, snapshotID, nodeID string) (*ingest.Node, error) {
	var node ingest.Node
	var labelsJSON, propsJSON, kind string

	err := s.db.QueryRowContext(ctx,
		"SELECT id, kind, labels, props FROM nodes WHERE snapshot_id = ? AND id = ?",
		snapshotID, nodeID,
	).Scan(&node.ID, &kind, &labelsJSON, &propsJSON)

	if err != nil {
		return nil, err
	}

	node.Kind = ingest.Kind(kind)
	if err := json.Unmarshal([]byte(labelsJSON), &node.Labels); err != nil {
		return nil, fmt.Errorf("unmarshaling labels: %w", err)
	}
	if err := json.Unmarshal([]byte(propsJSON), &node.Props); err != nil {
		return nil, fmt.Errorf("unmarshaling props: %w", err)
	}

	return &node, nil
}

// GetEdges retrieves all edges for a snapshot (ordered for determinism)
func (s *Store) GetEdges(ctx context.Context, snapshotID string) ([]ingest.Edge, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT src, dst, kind, props FROM edges WHERE snapshot_id = ? ORDER BY src, dst, kind",
		snapshotID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []ingest.Edge
	for rows.Next() {
		var edge ingest.Edge
		var propsJSON string

		if err := rows.Scan(&edge.Src, &edge.Dst, &edge.Kind, &propsJSON); err != nil {
			return nil, err
		}

		if err := json.Unmarshal([]byte(propsJSON), &edge.Props); err != nil {
			return nil, fmt.Errorf("unmarshaling edge props: %w", err)
		}
		edges = append(edges, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return edges, nil
}
