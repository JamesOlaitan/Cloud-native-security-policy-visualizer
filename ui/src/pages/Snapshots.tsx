import React, { useState, useEffect } from 'react'
import { gql, useQuery } from '@apollo/client'
import DiffLegend from '../components/DiffLegend'
import { Snapshot, GraphEdge, SnapshotDiff } from '../types'

const SNAPSHOTS_QUERY = gql`
  query GetSnapshots {
    snapshots {
      id
      createdAt
      label
      nodeCount
      edgeCount
    }
  }
`

const SNAPSHOT_DIFF = gql`
  query GetSnapshotDiff($a: ID!, $b: ID!) {
    snapshotDiff(a: $a, b: $b) {
      addedEdges {
        from
        to
        kind
      }
      removedEdges {
        from
        to
        kind
      }
      summary {
        added
        removed
        changed
      }
    }
  }
`

export default function Snapshots() {
  const [snapshotA, setSnapshotA] = useState('')
  const [snapshotB, setSnapshotB] = useState('')

  const { data: snapshotsData } = useQuery<{ snapshots: Snapshot[] }>(SNAPSHOTS_QUERY)
  const { loading, error, data } = useQuery<{ snapshotDiff: SnapshotDiff }>(SNAPSHOT_DIFF, {
    variables: { a: snapshotA, b: snapshotB },
    skip: !snapshotA || !snapshotB,
  })

  useEffect(() => {
    if (snapshotsData?.snapshots?.length && snapshotsData.snapshots.length >= 2) {
      if (!snapshotA) setSnapshotA(snapshotsData.snapshots[1].id)
      if (!snapshotB) setSnapshotB(snapshotsData.snapshots[0].id)
    }
  }, [snapshotsData, snapshotA, snapshotB])

  const snapshots = snapshotsData?.snapshots || []
  const diff = data?.snapshotDiff

  return (
    <div className="snapshots-container">
      <h2 className="page-title">Snapshot Comparison</h2>

      <div className="snapshot-selectors">
        <div>
          <label>Snapshot A (older):</label>
          <select
            value={snapshotA}
            onChange={(e) => setSnapshotA(e.target.value)}
          >
            <option value="">Select snapshot...</option>
            {snapshots.map((snap) => (
              <option key={snap.id} value={snap.id}>
                {snap.id} - {snap.label || 'No label'}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label>Snapshot B (newer):</label>
          <select
            value={snapshotB}
            onChange={(e) => setSnapshotB(e.target.value)}
          >
            <option value="">Select snapshot...</option>
            {snapshots.map((snap) => (
              <option key={snap.id} value={snap.id}>
                {snap.id} - {snap.label || 'No label'}
              </option>
            ))}
          </select>
        </div>
      </div>

      {loading && <div className="loading">Computing diff...</div>}
      {error && <div className="error">Error: {error.message}</div>}

      {diff && (
        <>
          <DiffLegend />

          <div className="diff-summary">
            <h3>Summary</h3>
            <p>Added: {diff.summary.added}</p>
            <p>Removed: {diff.summary.removed}</p>
          </div>

          <div className="diff-edges">
            {diff.addedEdges.length > 0 && (
              <>
                <h3>Added Edges</h3>
                {diff.addedEdges.map((edge, i) => (
                  <div key={i} className="edge-item edge-added">
                    {edge.from} --[{edge.kind}]-&gt; {edge.to}
                  </div>
                ))}
              </>
            )}

            {diff.removedEdges.length > 0 && (
              <>
                <h3 style={{ marginTop: '1rem' }}>Removed Edges</h3>
                {diff.removedEdges.map((edge, i) => (
                  <div key={i} className="edge-item edge-removed">
                    {edge.from} --[{edge.kind}]-&gt; {edge.to}
                  </div>
                ))}
              </>
            )}
          </div>
        </>
      )}
    </div>
  )
}
