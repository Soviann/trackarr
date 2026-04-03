import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import Router from 'preact-router'
import { Navbar } from './components/Navbar'
import { FilterBar, FilterTab } from './components/FilterBar'
import { Library } from './pages/Library'
import { TitleDetail } from './pages/TitleDetail'
import { Search } from './pages/Search'
import { Add } from './pages/Add'
import { Validate } from './pages/Validate'
import { MatchReview } from './pages/MatchReview'
import { Login } from './pages/Login'

function Placeholder({ name }: { name: string; path?: string }) {
  return (
    <div style={{ padding: '20px' }}>
      <h1 style={{ fontSize: '20px', fontWeight: 700 }}>{name}</h1>
      <p style={{ color: '#666', marginTop: '8px' }}>Coming soon.</p>
    </div>
  )
}

export function App() {
  const [currentPath, setCurrentPath] = useState(window.location.pathname)
  const [filterTab, setFilterTab] = useState<FilterTab>('all')

  const handleRoute = (e: { url: string }) => {
    setCurrentPath(e.url)
  }

  const navigate = (path: string) => {
    route(path)
  }

  const hideNavbar = currentPath === '/login' || currentPath.startsWith('/validate')
  const showFilterBar = currentPath === '/'

  return (
    <div style={{ minHeight: '100vh', paddingBottom: hideNavbar ? 0 : '108px' }}>
      <Router onChange={handleRoute}>
        <Library path="/" filterTab={filterTab} />
        <Search path="/search" />
        <Add path="/add" />
        <Placeholder name="Stats" path="/stats" />
        <Login path="/login" />
        <TitleDetail path="/title/:id" />
        <Validate path="/validate" />
        <MatchReview path="/match-review" />
      </Router>
      {!hideNavbar && (
        <Navbar
          currentPath={currentPath}
          onNavigate={navigate}
          above={showFilterBar ? <FilterBar active={filterTab} onChange={setFilterTab} /> : undefined}
        />
      )}
    </div>
  )
}
