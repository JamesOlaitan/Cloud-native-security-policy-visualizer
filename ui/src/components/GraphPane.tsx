import React, { useEffect, useRef } from 'react'
import cytoscape from 'cytoscape'
import { GraphNode, Neighbor, Path } from '../types'

interface GraphPaneProps {
  centerNode: GraphNode
  neighbors: Neighbor[]
  pathData?: Path
  onNodeClick: (node: { id: string; kind: string; labels?: string[]; props?: { key: string; value: string }[] }) => void
}

export default function GraphPane({ centerNode, neighbors, pathData, onNodeClick }: GraphPaneProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const cyRef = useRef<cytoscape.Core | null>(null)

  useEffect(() => {
    if (!containerRef.current) return

    // Build cytoscape elements
    const elements: cytoscape.ElementDefinition[] = []

    // Add center node
    elements.push({
      data: {
        ...centerNode,
        id: centerNode.id,
        label: centerNode.labels.join(', '),
        kind: centerNode.kind,
      },
    })

    // Add neighbors
    neighbors.forEach((neighbor) => {
      elements.push({
        data: {
          ...neighbor,
          id: neighbor.id,
          label: neighbor.labels.join(', '),
          kind: neighbor.kind,
        },
      })

      // Add edge
      elements.push({
        data: {
          id: `${centerNode.id}-${neighbor.id}`,
          source: centerNode.id,
          target: neighbor.id,
          label: neighbor.edgeKind,
        },
      })
    })

    // Initialize cytoscape
    const cy = cytoscape({
      container: containerRef.current,
      elements,
      style: [
        {
          selector: 'node',
          style: {
            'background-color': '#3498db',
            'label': 'data(label)',
            'text-valign': 'center',
            'color': '#fff',
            'text-outline-width': 2,
            'text-outline-color': '#3498db',
            'width': 60,
            'height': 60,
            'font-size': 10,
          },
        },
        {
          selector: 'node[kind="PRINCIPAL"]',
          style: {
            'background-color': '#2ecc71',
            'text-outline-color': '#2ecc71',
          },
        },
        {
          selector: 'node[kind="RESOURCE"]',
          style: {
            'background-color': '#e74c3c',
            'text-outline-color': '#e74c3c',
          },
        },
        {
          selector: 'node[kind="POLICY"]',
          style: {
            'background-color': '#f39c12',
            'text-outline-color': '#f39c12',
          },
        },
        {
          selector: 'edge',
          style: {
            'width': 2,
            'line-color': '#95a5a6',
            'target-arrow-color': '#95a5a6',
            'target-arrow-shape': 'triangle',
            'curve-style': 'bezier',
            'label': 'data(label)',
            'font-size': 8,
            'text-rotation': 'autorotate',
          },
        },
        {
          selector: '.highlighted',
          style: {
            'background-color': '#9b59b6',
            'line-color': '#9b59b6',
            'target-arrow-color': '#9b59b6',
            'width': 4,
          },
        },
      ],
      layout: {
        name: 'dagre',
        rankDir: 'LR',
        nodeSep: 50,
        rankSep: 100,
      } as cytoscape.LayoutOptions,
    })

    // Add click handler
    cy.on('tap', 'node', (evt) => {
      const node = evt.target
      onNodeClick(node.data())
    })

    cyRef.current = cy

    return () => {
      cy.destroy()
    }
  }, [centerNode, neighbors, onNodeClick])

  // Highlight path
  useEffect(() => {
    if (!cyRef.current || !pathData) return

    const cy = cyRef.current

    // Remove previous highlights
    cy.elements().removeClass('highlighted')

    // Highlight path nodes and edges
    const pathNodeIds = pathData.nodes.map((n) => n.id)
    pathNodeIds.forEach((id) => {
      cy.getElementById(id).addClass('highlighted')
    })

    pathData.edges.forEach((edge) => {
      const cyEdge = cy.edges().filter((e) => {
        const data = e.data()
        return data.source === edge.from && data.target === edge.to
      })
      cyEdge.addClass('highlighted')
    })
  }, [pathData])

  return <div ref={containerRef} className="graph-canvas" style={{ width: '100%', height: '100%' }} />
}
