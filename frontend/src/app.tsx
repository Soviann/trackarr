import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import Router from 'preact-router'
import { Navbar } from './components/Navbar'
import { Library } from './pages/Library'
import { TitleDetail } from './pages/TitleDetail'

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

  const handleRoute = (e: { url: string }) => {
    setCurrentPath(e.url)
  }

  const navigate = (path: string) => {
    route(path)
  }

  // Hide navbar on login and validate pages
  const hideNavbar = currentPath === '/login' || currentPath.startsWith('/validate')

  return (
    <div style={{ minHeight: '100vh', paddingBottom: hideNavbar ? 0 : '108px' }}>
      <Router onChange={handleRoute}>
        <Library path="/" />
        <Placeholder name="Search" path="/search" />
        <Placeholder name="Add" path="/add" />
        <Placeholder name="Stats" path="/stats" />
        <Placeholder name="Login" path="/login" />
        <TitleDetail path="/title/:id" />
        <Placeholder name="Validate" path="/validate" />
        <Placeholder name="Match Review" path="/match-review" />
      </Router>
      {!hideNavbar && <Navbar currentPath={currentPath} onNavigate={navigate} />}
    </div>
  )
}
