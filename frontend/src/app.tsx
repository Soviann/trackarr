import { useState, useEffect, useCallback } from 'preact/hooks'
import { route } from 'preact-router'
import Router from 'preact-router'
import clsx from 'clsx'
import { Navbar } from './components/Navbar'
import { FilterDrawer } from './components/FilterDrawer'
import { SearchBar } from './components/SearchBar'
import { Library } from './pages/Library'
import { ComingUp } from './pages/ComingUp'
import { ContinueWatching } from './pages/ContinueWatching'
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
import { AdminAniList } from './pages/AdminAniList'
import { AnilistCallback } from './pages/AnilistCallback'
import { Help } from './pages/Help'
import { PersonTitles } from './pages/PersonTitles'
import { usePush } from './hooks/usePush'
import { updateBadge } from './utils/badge'
import { useTitleStore, useSearchStore } from './store'
import { ErrorBoundary } from './components/ErrorBoundary'
import { ROUTE_PATHS } from './routes'
import s from './app.module.css'
import type { TitleStatus, TitleType, SeriesStatus } from './types'

type StatusFilter = TitleStatus | 'up_to_date' | null
type TypeFilter = TitleType | null
type SeriesStatusFilter = SeriesStatus | null

const defaultFilter = { status: undefined, type: undefined, series_status: undefined, search: undefined, decade: undefined, release_from: undefined, release_to: undefined, include_no_release: undefined, genres: undefined, genre_op: undefined }

export function App() {
  const [currentPath, setCurrentPath] = useState(window.location.pathname)
  const [vapidKey, setVapidKey] = useState<string>()

  useEffect(() => {
    fetch('/api/config').then(r => r.json()).then(cfg => {
      if (cfg.vapid_public_key) setVapidKey(cfg.vapid_public_key)
    }).catch((err) => {
      console.error('Failed to load config:', err)
    })
  }, [])

  const isAuthed = currentPath !== '/login'
  useEffect(() => { if (isAuthed) updateBadge() }, [isAuthed])

  usePush(isAuthed ? vapidKey : undefined)

  const handleRoute = (e: { url: string }) => {
    setCurrentPath(e.url)
  }

  const filter = useTitleStore(s => s.filter)
  const setFilter = useTitleStore(s => s.setFilter)
  const sort = useTitleStore(s => s.sort)
  const setSort = useTitleStore(s => s.setSort)

  const navigate = useCallback((path: string) => {
    if (path === '/' || path === '/search') {
      setFilter(defaultFilter)
      useSearchStore.getState().clear()
    }
    route(path)
  }, [setFilter])

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

  const handleGenreToggle = useCallback((genre: string) => {
    const current = filter.genres ?? []
    const next = current.includes(genre) ? current.filter(g => g !== genre) : [...current, genre]
    setFilter({ genres: next.length > 0 ? next : undefined })
  }, [filter.genres, setFilter])

  const handleGenreOpChange = useCallback((op: 'AND' | 'OR') => {
    setFilter({ genre_op: op })
  }, [setFilter])
  const hideNavbar = currentPath.startsWith('/login')
  const pathname = currentPath.split('?')[0]
  const isSearch = pathname === '/search'
  const showDrawer = pathname === '/' || isSearch
  const mergeSourceId = isSearch
    ? new URLSearchParams(currentPath.split('?')[1] ?? '').get('mergeSourceId')
    : null

  const filterDrawer = showDrawer ? (
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
      isSearchActive={isSearch}
      defaultOpen={true}
      decade={filter.decade ?? null}
      releaseFrom={filter.release_from ?? ''}
      releaseTo={filter.release_to ?? ''}
      includeNoRelease={filter.include_no_release !== 'false'}
      onDecadeChange={handleDecadeChange}
      onReleaseFromChange={handleReleaseFromChange}
      onReleaseToChange={handleReleaseToChange}
      onIncludeNoReleaseChange={handleIncludeNoReleaseChange}
      selectedGenres={filter.genres ?? []}
      genreOp={filter.genre_op ?? 'OR'}
      onGenreToggle={handleGenreToggle}
      onGenreOpChange={handleGenreOpChange}
    />
  ) : null

  const above = isSearch ? (
    <>
      <SearchBar showTMDBToggle={!mergeSourceId} />
      {filterDrawer}
    </>
  ) : filterDrawer

  return (
    <ErrorBoundary>
      <div className={clsx(s.root, !hideNavbar && s.withNavbar, isSearch && s.withSearchBar)}>
        <Router onChange={handleRoute}>
          <Library path={ROUTE_PATHS.home} />
          <ComingUp path={ROUTE_PATHS.comingUp} />
          <ContinueWatching path={ROUTE_PATHS.continueWatching} />
          <Search path={ROUTE_PATHS.search} />
          <Add path={ROUTE_PATHS.add} />
          <Stats path={ROUTE_PATHS.stats} />
          <Login path={ROUTE_PATHS.login} />
          <TitleDetail path={ROUTE_PATHS.title} />
          <PersonTitles path={ROUTE_PATHS.person} />
          <Admin path={ROUTE_PATHS.admin} />
          <Validate path={ROUTE_PATHS.adminValidate} />
          <AdminTasks path={ROUTE_PATHS.adminTasks} />
          <AdminNotifications path={ROUTE_PATHS.adminNotifications} />
          <AdminAniList path={ROUTE_PATHS.adminAniList} />
          <Help path={ROUTE_PATHS.adminHelp} />
          <AnilistCallback path={ROUTE_PATHS.anilistCallback} />
          <MatchReview path={ROUTE_PATHS.matchReview} />
        </Router>
        {!hideNavbar && (
          <Navbar
            currentPath={currentPath}
            onNavigate={navigate}
            above={above}
          />
        )}
      </div>
    </ErrorBoundary>
  )
}
