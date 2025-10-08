import React, { useState } from 'react'

interface SearchBarProps {
  onSearch: (query: string) => void
}

export default function SearchBar({ onSearch }: SearchBarProps) {
  const [query, setQuery] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSearch(query)
  }

  return (
    <form onSubmit={handleSubmit}>
      <input
        type="text"
        className="search-box"
        placeholder="Search for principals (e.g., DevRole, sa-ci)..."
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />
    </form>
  )
}

