import { useState, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import Router from 'preact-router'
import clsx from 'clsx'
import { Navbar } from './components/Navbar'
import { FilterBar, FilterTab } from './components/FilterBar'
import { Library } from './pages/Library'
import { TitleDetail } from './pages/TitleDetail'
import { Search } from './pages/Search'
import { Add } from './pages/Add'
import { Validate } from './pages/Validate'
import { MatchReview } from './pages/MatchReview'
import { Login } from './pages/Login'
import { Stats } from './pages/Stats'
import { usePush } from './hooks/usePush'
import { ErrorBoundary } from './components/ErrorBoundary'
import s from './app.module.css'

export function App() {
  const [currentPath, setCurrentPath] = useState(window.location.pathname)
  const [filterTab, setFilterTab] = useState<FilterTab>('watching')
  const [vapidKey, setVapidKey] = useState<string>()

  useEffect(() => {
    fetch('/api/config').then(r => r.json()).then(cfg => {
      if (cfg.vapid_public_key) setVapidKey(cfg.vapid_public_key)
    })
  }, [])

  usePush(currentPath !== '/login' ? vapidKey : undefined)

  const handleRoute = (e: { url: string }) => {
    setCurrentPath(e.url)
  }

  const navigate = (path: string) => {
    route(path)
  }

  const hideNavbar = currentPath === '/login' || currentPath.startsWith('/validate')
  const showFilterBar = currentPath === '/'

  return (
    <ErrorBoundary>
      <div className={clsx(s.root, !hideNavbar && s.withNavbar)}>
        <Router onChange={handleRoute}>
          <Library path="/" filterTab={filterTab} />
          <Search path="/search" />
          <Add path="/add" />
          <Stats path="/stats" />
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
    </ErrorBoundary>
  )
}
