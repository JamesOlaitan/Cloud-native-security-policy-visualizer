import React, { useEffect, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { gql, useQuery, useLazyQuery } from '@apollo/client'
import cytoscape from 'cytoscape'
import dagre from 'cytoscape-dagre'
import GraphPane from '../components/GraphPane'

cytoscape.use(dagre)

const NODE_QUERY = gql`
  query GetNode($id: ID!) {
    node(id: $id) {
      id
      kind
      labels
      props {
        key
        value
      }
      neighbors {
        id
        kind
        labels
        edgeKind
      }
    }
  }
`

const SHORTEST_PATH = gql`
  query ShortestPath($from: ID!, $to: ID!, $maxHops: Int) {
    shortestPath(from: $from, to: $to, maxHops: $maxHops) {
      nodes {
        id
        kind
        labels
      }
      edges {
        from
        to
        kind
      }
    }
  }
`

export default function GraphView() {
  const { nodeId } = useParams<{ nodeId: string }>()
  const decodedNodeId = nodeId ? decodeURIComponent(nodeId) : ''
  const [selectedNode, setSelectedNode] = useState<any>(null)
  const [targetResource, setTargetResource] = useState('')
  const [pathData, setPathData] = useState<any>(null)

  const { loading, error, data } = useQuery(NODE_QUERY, {
    variables: { id: decodedNodeId },
    skip: !decodedNodeId,
  })

  const [findPath, { loading: pathLoading }] = useLazyQuery(SHORTEST_PATH, {
    onCompleted: (pathResult) => {
      setPathData(pathResult.shortestPath)
    },
  })

  const handleNodeClick = (node: any) => {
    setSelectedNode(node)
  }

  const handleFindPath = () => {
    if (targetResource && decodedNodeId) {
      findPath({
        variables: {
          from: decodedNodeId,
          to: targetResource,
          maxHops: 8,
        },
      })
    }
  }

  if (loading) return <div className="loading">Loading graph...</div>
  if (error) return <div className="error">Error: {error.message}</div>
  if (!data?.node) return <div className="error">Node not found</div>

  const node = data.node
  const neighbors = node.neighbors || []

  // Extract resource nodes for path finding
  const resourceNodes = neighbors.filter((n: any) => n.kind === 'RESOURCE')

  return (
    <div>
      <h2 className="page-title">Graph View: {node.labels.join(', ')}</h2>
      
      <div className="graph-container">
        <GraphPane
          centerNode={node}
          neighbors={neighbors}
          pathData={pathData}
          onNodeClick={handleNodeClick}
        />
        
        <div className="graph-sidebar">
          <div className="path-controls">
            <h3>Find Path</h3>
            <select
              value={targetResource}
              onChange={(e) => setTargetResource(e.target.value)}
            >
              <option value="">Select resource...</option>
              {resourceNodes.map((n: any) => (
                <option key={n.id} value={n.id}>
                  {n.labels.join(', ')}
                </option>
              ))}
            </select>
            <button onClick={handleFindPath} disabled={!targetResource || pathLoading}>
              {pathLoading ? 'Finding...' : 'Find Path'}
            </button>
          </div>
          
          {selectedNode && (
            <div className="node-details">
              <h3>Node Details</h3>
              <ul>
                <li><strong>ID:</strong> {selectedNode.id}</li>
                <li><strong>Kind:</strong> {selectedNode.kind}</li>
                <li><strong>Labels:</strong> {selectedNode.labels?.join(', ')}</li>
                {selectedNode.props?.map((prop: any) => (
                  <li key={prop.key}>
                    <strong>{prop.key}:</strong> {prop.value}
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

