/** Shared TypeScript types mirroring the GraphQL schema. */

export interface KV {
  key: string
  value: string
}

export interface GraphNode {
  id: string
  kind: string
  labels: string[]
  props: KV[]
  neighbors?: Neighbor[]
}

export interface GraphEdge {
  from: string
  to: string
  kind: string
}

export interface Neighbor {
  id: string
  kind: string
  labels: string[]
  edgeKind: string
}

export interface Path {
  nodes: GraphNode[]
  edges: GraphEdge[]
}

export interface Finding {
  id: string
  ruleId: string
  severity: string
  entityRef: string
  reason: string
  remediation: string
}

export interface Snapshot {
  id: string
  createdAt: string
  label: string | null
  nodeCount: number
  edgeCount: number
}

export interface DiffSummary {
  added: number
  removed: number
  changed: number
}

export interface SnapshotDiff {
  addedEdges: GraphEdge[]
  removedEdges: GraphEdge[]
  summary: DiffSummary
}
