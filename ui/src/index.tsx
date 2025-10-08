import React from 'react'
import ReactDOM from 'react-dom/client'
import { BrowserRouter, Routes, Route, Link } from 'react-router-dom'
import { ApolloProvider } from '@apollo/client'
import { client } from './apollo'
import Search from './pages/Search'
import GraphView from './pages/GraphView'
import Findings from './pages/Findings'
import Snapshots from './pages/Snapshots'
import './styles.css'

function App() {
  return (
    <ApolloProvider client={client}>
      <BrowserRouter>
        <div className="app">
          <nav className="navbar">
            <div className="nav-brand">
              <h1>AccessGraph</h1>
            </div>
            <div className="nav-links">
              <Link to="/">Search</Link>
              <Link to="/findings">Findings</Link>
              <Link to="/snapshots">Snapshots</Link>
            </div>
          </nav>
          <div className="content">
            <Routes>
              <Route path="/" element={<Search />} />
              <Route path="/graph/:nodeId" element={<GraphView />} />
              <Route path="/findings" element={<Findings />} />
              <Route path="/snapshots" element={<Snapshots />} />
            </Routes>
          </div>
        </div>
      </BrowserRouter>
    </ApolloProvider>
  )
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
)

