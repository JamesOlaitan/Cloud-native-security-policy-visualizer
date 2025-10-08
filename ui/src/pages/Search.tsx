import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { gql, useLazyQuery } from '@apollo/client'
import SearchBar from '../components/SearchBar'

const SEARCH_PRINCIPALS = gql`
  query SearchPrincipals($query: String!, $limit: Int) {
    searchPrincipals(query: $query, limit: $limit) {
      id
      kind
      labels
      props {
        key
        value
      }
    }
  }
`

interface Node {
  id: string
  kind: string
  labels: string[]
  props: { key: string; value: string }[]
}

export default function Search() {
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [searchPrincipals, { loading, error, data }] = useLazyQuery(SEARCH_PRINCIPALS)

  const handleSearch = (searchQuery: string) => {
    setQuery(searchQuery)
    if (searchQuery.trim()) {
      searchPrincipals({ variables: { query: searchQuery, limit: 10 } })
    }
  }

  const handleSelectNode = (nodeId: string) => {
    navigate(`/graph/${encodeURIComponent(nodeId)}`)
  }

  const results: Node[] = data?.searchPrincipals || []

  return (
    <div>
      <h2 className="page-title">Search Principals</h2>
      <SearchBar onSearch={handleSearch} />
      
      {loading && <div className="loading">Searching...</div>}
      {error && <div className="error">Error: {error.message}</div>}
      
      {results.length > 0 && (
        <ul className="results-list">
          {results.map((node) => (
            <li
              key={node.id}
              className="result-item"
              onClick={() => handleSelectNode(node.id)}
            >
              <div className="result-title">{node.labels.join(', ')}</div>
              <div className="result-meta">
                {node.kind} - {node.id}
              </div>
            </li>
          ))}
        </ul>
      )}
      
      {query && results.length === 0 && !loading && (
        <div>No results found for "{query}"</div>
      )}
    </div>
  )
}

