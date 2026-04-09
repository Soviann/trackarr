import { useState, useEffect, useCallback } from 'preact/hooks'
import { route } from 'preact-router'
import Router from 'preact-router'
import clsx from 'clsx'
import { Navbar } from './components/Navbar'
import { FilterDrawer } from './components/FilterDrawer'
import { Library } from './pages/Library'
import { TitleDetail } from './pages/TitleDetail'
import { Search } from './pages/Search'
import { Add } from './pages/Add'
import { Validate } from './pages/Validate'
import { MatchReview } from './pages/MatchReview'
import { Login } from './pages/Login'
import { Stats } from './pages/Stats'
import { Admin } from './pages/Admin'
import { AdminTasks } from './pages/AdminTasks'
import { AdminNotifications } from './pages/AdminNotifications'
import { usePush } from './hooks/usePush'
import { useTitleStore, useSearchStore } from './store'
import { ErrorBoundary } from './components/ErrorBoundary'
import s from './app.module.css'
import type { TitleStatus, TitleType, SeriesStatus } from './types'

type StatusFilter = TitleStatus | 'up_to_date' | null
type TypeFilter = TitleType | null
type SeriesStatusFilter = SeriesStatus | null

export function App() {
  const [currentPath, setCurrentPath] = useState(window.location.pathname)
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

  const defaultFilter = { status: undefined, type: undefined, series_status: undefined, search: undefined, decade: undefined, release_from: undefined, release_to: undefined, include_no_release: undefined }

  const navigate = (path: string) => {
    if (path === '/' || path === '/search') {
      setFilter(defaultFilter)
      useSearchStore.getState().clear()
    }
    route(path)
  }

  const { filter, setFilter, sort, setSort } = useTitleStore()

  const statusFilter: StatusFilter = filter.status === 'watching_behind'
    ? 'watching'
    : filter.status === 'up_to_date'
      ? 'up_to_date'
      : (filter.status as TitleStatus | undefined) ?? null
  const typeFilter: TypeFilter = (filter.type as TitleType | undefined) ?? null
  const seriesStatusFilter: SeriesStatusFilter = (filter.series_status as SeriesStatus | undefined) ?? null

  const handleStatusChange = useCallback((s: StatusFilter) => {
    let storeStatus: string | undefined
    if (s === 'watching') storeStatus = 'watching_behind'
    else if (s === 'up_to_date') storeStatus = 'up_to_date'
    else if (s === null) storeStatus = undefined
    else storeStatus = s
    setFilter({ status: storeStatus })
  }, [setFilter])

  const handleTypeChange = useCallback((t: TypeFilter) => {
    const updates: Record<string, string | undefined> = { type: t ?? undefined }
    if (t !== 'series') updates.series_status = undefined
    setFilter(updates)
  }, [setFilter])

  const handleIsAnimeChange = useCallback((ia: boolean) => {
    setFilter({ is_anime: ia ? 'true' : undefined })
  }, [setFilter])

  const handleSeriesStatusChange = useCallback((ss: SeriesStatusFilter) => {
    setFilter({ series_status: ss ?? undefined })
  }, [setFilter])

  const handleDecadeChange = useCallback((d: string | null) => {
    setFilter({ decade: d ?? undefined, release_from: undefined, release_to: undefined })
  }, [setFilter])

  const handleReleaseFromChange = useCallback((d: string) => {
    setFilter({ release_from: d || undefined, decade: undefined })
  }, [setFilter])

  const handleReleaseToChange = useCallback((d: string) => {
    setFilter({ release_to: d || undefined, decade: undefined })
  }, [setFilter])

  const handleIncludeNoReleaseChange = useCallback((include: boolean) => {
    setFilter({ include_no_release: include ? undefined : 'false' })
  }, [setFilter])
  const hideNavbar = currentPath.startsWith('/login') || currentPath.startsWith('/admin/tasks')
  const pathname = currentPath.split('?')[0]
  const showDrawer = pathname === '/' || pathname === '/search'

  return (
    <ErrorBoundary>
      <div className={clsx(s.root, !hideNavbar && s.withNavbar)}>
        <Router onChange={handleRoute}>
          <Library path="/" />
          <Search path="/search" />
          <Add path="/add" />
          <Stats path="/stats" />
          <Login path="/login" />
          <TitleDetail path="/title/:id" />
          <Admin path="/admin" />
          <Validate path="/admin/validate" />
          <AdminTasks path="/admin/tasks" />
          <AdminNotifications path="/admin/notifications" />
          <MatchReview path="/match-review" />
        </Router>
        {!hideNavbar && (
          <Navbar
            currentPath={currentPath}
            onNavigate={navigate}
            above={showDrawer ? (
              <FilterDrawer
                status={statusFilter}
                type={typeFilter}
                isAnime={filter.is_anime === 'true'}
                seriesStatus={seriesStatusFilter}
                onStatusChange={handleStatusChange}
                onTypeChange={handleTypeChange}
                onIsAnimeChange={handleIsAnimeChange}
                onSeriesStatusChange={handleSeriesStatusChange}
                sort={sort}
                onSortChange={setSort}
                isSearchActive={pathname === '/search'}
                defaultOpen={true}
                decade={filter.decade ?? null}
                releaseFrom={filter.release_from ?? ''}
                releaseTo={filter.release_to ?? ''}
                includeNoRelease={filter.include_no_release !== 'false'}
                onDecadeChange={handleDecadeChange}
                onReleaseFromChange={handleReleaseFromChange}
                onReleaseToChange={handleReleaseToChange}
                onIncludeNoReleaseChange={handleIncludeNoReleaseChange}
              />
            ) : undefined}
          />
        )}
      </div>
    </ErrorBoundary>
  )
}
