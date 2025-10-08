CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    label TEXT
);

CREATE TABLE IF NOT EXISTS nodes (
    snapshot_id TEXT NOT NULL,
    id TEXT NOT NULL,
    kind TEXT NOT NULL,
    labels TEXT NOT NULL,
    props TEXT NOT NULL,
    FOREIGN KEY (snapshot_id) REFERENCES snapshots(id)
);

CREATE TABLE IF NOT EXISTS edges (
    snapshot_id TEXT NOT NULL,
    src TEXT NOT NULL,
    dst TEXT NOT NULL,
    kind TEXT NOT NULL,
    props TEXT NOT NULL,
    FOREIGN KEY (snapshot_id) REFERENCES snapshots(id)
);

CREATE INDEX IF NOT EXISTS idx_nodes_snapshot ON nodes(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_edges_snapshot ON edges(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_nodes_id ON nodes(snapshot_id, id);
CREATE INDEX IF NOT EXISTS idx_nodes_kind ON nodes(snapshot_id, kind);

