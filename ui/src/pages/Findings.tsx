import React, { useEffect, useState } from 'react'
import { gql, useQuery } from '@apollo/client'

const SNAPSHOTS_QUERY = gql`
  query GetSnapshots {
    snapshots {
      id
      createdAt
      label
    }
  }
`

const FINDINGS_QUERY = gql`
  query GetFindings($snapshotId: ID!) {
    findings(snapshotId: $snapshotId) {
      id
      ruleId
      severity
      entityRef
      reason
      remediation
    }
  }
`

export default function Findings() {
  const [selectedSnapshot, setSelectedSnapshot] = useState('')

  const { data: snapshotsData } = useQuery(SNAPSHOTS_QUERY)
  const { loading, error, data } = useQuery(FINDINGS_QUERY, {
    variables: { snapshotId: selectedSnapshot },
    skip: !selectedSnapshot,
  })

  useEffect(() => {
    if (snapshotsData?.snapshots?.length > 0 && !selectedSnapshot) {
      setSelectedSnapshot(snapshotsData.snapshots[0].id)
    }
  }, [snapshotsData, selectedSnapshot])

  const findings = data?.findings || []

  return (
    <div>
      <h2 className="page-title">Policy Findings</h2>
      
      {snapshotsData?.snapshots && (
        <div style={{ marginBottom: '1rem' }}>
          <select
            value={selectedSnapshot}
            onChange={(e) => setSelectedSnapshot(e.target.value)}
            style={{ padding: '0.5rem', fontSize: '1rem' }}
          >
            {snapshotsData.snapshots.map((snap: any) => (
              <option key={snap.id} value={snap.id}>
                {snap.id} - {snap.label || 'No label'}
              </option>
            ))}
          </select>
        </div>
      )}
      
      {loading && <div className="loading">Loading findings...</div>}
      {error && <div className="error">Error: {error.message}</div>}
      
      {findings.length > 0 && (
        <table className="findings-table">
          <thead>
            <tr>
              <th>Rule ID</th>
              <th>Severity</th>
              <th>Entity</th>
              <th>Reason</th>
              <th>Remediation</th>
            </tr>
          </thead>
          <tbody>
            {findings.map((finding: any) => (
              <tr key={finding.id}>
                <td>{finding.ruleId}</td>
                <td className={`severity-${finding.severity}`}>{finding.severity}</td>
                <td style={{ fontSize: '0.75rem', wordBreak: 'break-all' }}>
                  {finding.entityRef}
                </td>
                <td>{finding.reason}</td>
                <td style={{ fontSize: '0.875rem' }}>{finding.remediation}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      
      {selectedSnapshot && findings.length === 0 && !loading && (
        <div>No findings for this snapshot</div>
      )}
    </div>
  )
}

