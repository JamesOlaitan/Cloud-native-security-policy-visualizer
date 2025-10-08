import React from 'react'

export default function DiffLegend() {
  return (
    <div className="diff-legend">
      <div className="legend-item">
        <div className="legend-color legend-added"></div>
        <span>Added</span>
      </div>
      <div className="legend-item">
        <div className="legend-color legend-removed"></div>
        <span>Removed</span>
      </div>
    </div>
  )
}

