import { useState, useEffect, useCallback, useRef } from 'preact/hooks'
import { route } from 'preact-router'
import Router from 'preact-router'
import clsx from 'clsx'
import { Navbar } from './components/Navbar'
import { Sidebar } from './components/Sidebar'
import { FilterDrawer, type FilterState, type FilterActions } from './components/FilterDrawer'
import { SearchBar } from './components/SearchBar'
import { useKeyboardShortcuts } from './hooks/useKeyboardShortcuts'
import { Library } from './pages/Library'
import { Calendar } from './pages/Calendar'
import { ComingUp } from './pages/ComingUp'
import { ContinueWatching } from './pages/ContinueWatching'
import { Releases } from './pages/Releases'
import { TitleDetail } from './pages/TitleDetail'
import { lazy, Suspense } from 'preact/compat'
import { Search } from './pages/Search'
import { Add } from './pages/Add'
import { Login } from './pages/Login'
import { Setup } from './pages/Setup'
import { AnilistCallback } from './pages/AnilistCallback'
import { PersonTitles } from './pages/PersonTitles'
import { usePush } from './hooks/usePush'
import { useActivePlayback } from './hooks/useActivePlayback'
import { LivePlaybackIndicator } from './components/LivePlaybackIndicator'
import { LivePlaybackDrawer } from './components/LivePlaybackDrawer'

const Stats = lazy(() => import('./pages/Stats').then((m) => ({ default: m.Stats })))
const Wrapped = lazy(() => import('./pages/Wrapped').then((m) => ({ default: m.Wrapped })))
const Admin = lazy(() => import('./pages/Admin').then((m) => ({ default: m.Admin })))
const AdminSettings = lazy(() => import('./pages/AdminSettings').then((m) => ({ default: m.AdminSettings })))
const AdminAuth = lazy(() => import('./pages/AdminAuth').then((m) => ({ default: m.AdminAuth })))
const AdminTasks = lazy(() => import('./pages/AdminTasks').then((m) => ({ default: m.AdminTasks })))
const AdminNotifications = lazy(() => import('./pages/AdminNotifications').then((m) => ({ default: m.AdminNotifications })))
const AdminJellyfin = lazy(() => import('./pages/AdminJellyfin').then((m) => ({ default: m.AdminJellyfin })))
const AdminAniList = lazy(() => import('./pages/AdminAniList').then((m) => ({ default: m.AdminAniList })))
const AdminArr = lazy(() => import('./pages/AdminArr').then((m) => ({ default: m.AdminArr })))
const AdminSeasonAudit = lazy(() => import('./pages/AdminSeasonAudit').then((m) => ({ default: m.AdminSeasonAudit })))
const Help = lazy(() => import('./pages/Help').then((m) => ({ default: m.Help })))
const Validate = lazy(() => import('./pages/Validate').then((m) => ({ default: m.Validate })))
const MatchReview = lazy(() => import('./pages/MatchReview').then((m) => ({ default: m.MatchReview })))
import { useServiceWorker } from './hooks/useServiceWorker'
import { useIsDesktop } from './hooks/useIsDesktop'
import { updateBadge } from './utils/badge'
import { setPreferredMetadataLanguage } from './utils'
import { setEnabledWatchProviders } from './utils/providers'
import { useTitleStore, useSearchStore } from './store'
import { ErrorBoundary } from './components/ErrorBoundary'
import { UndoProvider } from './context/UndoContext'
import { UndoSnackbar } from './components/UndoSnackbar'
import { ROUTE_PATHS } from './routes'
import s from './app.module.css'
import type { TitleStatus, TitleType, SeriesStatus } from './types'

type StatusFilter = TitleStatus | 'up_to_date' | null
type TypeFilter = TitleType | null
type SeriesStatusFilter = SeriesStatus | null

const isDetailPath = (path: string) => path.startsWith('/title/') || path.startsWith('/person/')

export function App() {
  const [currentPath, setCurrentPath] = useState(window.location.pathname)
  const [filterDrawerOpen, setFilterDrawerOpen] = useState(false)
  const [vapidKey, setVapidKey] = useState<string>()
  const fromSearchRef = useRef(false)

  useEffect(() => {
    fetch('/api/config').then(r => r.json()).then(cfg => {
      if (cfg.vapid_public_key) setVapidKey(cfg.vapid_public_key)
      if (cfg.metadata_language) setPreferredMetadataLanguage(cfg.metadata_language)
      if (cfg.enabled_watch_providers !== undefined) setEnabledWatchProviders(cfg.enabled_watch_providers)
    }).catch((err) => {
      console.error('Failed to load config:', err)
    })
  }, [])

  const isAuthed = currentPath !== '/login' && currentPath !== '/setup'
  useEffect(() => { if (isAuthed) updateBadge() }, [isAuthed])

  useServiceWorker(isAuthed)
  usePush(isAuthed ? vapidKey : undefined)

  const handleRoute = (e: { url: string }) => {
    const prevPathname = currentPath.split('?')[0]
    const nextPathname = e.url.split('?')[0]
    const prevIsMerge = new URLSearchParams(currentPath.split('?')[1] ?? '').has('mergeSourceId')
    const nextIsMerge = new URLSearchParams(e.url.split('?')[1] ?? '').has('mergeSourceId')

    const prevWasSearch = prevPathname === ROUTE_PATHS.search
    const nextIsSearch = nextPathname === ROUTE_PATHS.search
    const prevWasDetail = isDetailPath(prevPathname)
    const nextIsDetail = isDetailPath(nextPathname)

    if (prevWasSearch && nextIsDetail) {
      fromSearchRef.current = true
    } else if (prevWasDetail && nextIsDetail) {
      // Retain fromSearchRef across consecutive detail views (e.g. title -> person -> title)
    } else if (prevWasDetail && nextIsSearch && fromSearchRef.current) {
      fromSearchRef.current = false
    } else {
      const exitingSearch = (prevWasSearch && !nextIsSearch) || (fromSearchRef.current && !nextIsDetail && !nextIsSearch)
      const enteringFreshSearch = !prevWasSearch && nextIsSearch && !fromSearchRef.current
      const exitingMerge = prevIsMerge && !nextIsMerge

      if (exitingSearch || enteringFreshSearch || exitingMerge) {
        useSearchStore.getState().clear()
      }
      fromSearchRef.current = false
    }

    setCurrentPath(e.url)
    setFilterDrawerOpen(false)
    window.scrollTo(0, 0)
  }

  const filter = useTitleStore(s => s.filter)
  const setFilter = useTitleStore(s => s.setFilter)
  const sort = useTitleStore(s => s.sort)
  const setSort = useTitleStore(s => s.setSort)

  const navigate = useCallback((path: string) => {
    if (path === ROUTE_PATHS.search) {
      fromSearchRef.current = false
      if (currentPath.split('?')[0] === ROUTE_PATHS.search) {
        useSearchStore.getState().clear()
        window.scrollTo(0, 0)
        return
      }
    }
    route(path)
  }, [currentPath])

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
    setFilter({ release_from: d || undefined, decade: d ? undefined : filter.decade })
  }, [filter.decade, setFilter])

  const handleReleaseToChange = useCallback((d: string) => {
    setFilter({ release_to: d || undefined, decade: d ? undefined : filter.decade })
  }, [filter.decade, setFilter])

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

  const handleCountryToggle = useCallback((iso: string) => {
    const current = filter.origin_country ?? []
    const next = current.includes(iso) ? current.filter(c => c !== iso) : [...current, iso]
    setFilter({ origin_country: next.length > 0 ? next : undefined })
  }, [filter.origin_country, setFilter])

  const handleMyRatingMinChange = useCallback((v: string) => {
    setFilter({ my_rating_min: v || undefined })
  }, [setFilter])

  const handleTmdbRatingMinChange = useCallback((v: string) => {
    setFilter({ tmdb_rating_min: v || undefined })
  }, [setFilter])

  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(() => {
    try {
      return localStorage.getItem('trackarr_sidebar_collapsed') === 'true'
    } catch {
      return false
    }
  })

  const handleToggleSidebar = useCallback(() => {
    setSidebarCollapsed(prev => {
      const next = !prev
      try {
        localStorage.setItem('trackarr_sidebar_collapsed', String(next))
      } catch {
        // ignore
      }
      return next
    })
  }, [])

  useKeyboardShortcuts({
    onSearch: useCallback(() => {
      if (currentPath !== ROUTE_PATHS.search) {
        navigate(ROUTE_PATHS.search)
      } else {
        const searchInput = document.getElementById('search') as HTMLInputElement | null
        searchInput?.focus()
        searchInput?.select()
      }
    }, [currentPath, navigate]),
    onEscape: useCallback(() => {
      if (filterDrawerOpen) {
        setFilterDrawerOpen(false)
      }
    }, [filterDrawerOpen]),
    enabled: isAuthed,
  })

  const handleResetFilters = useCallback(() => {
    useTitleStore.setState({
      filter: {},
      titles: [],
      _fetchGen: useTitleStore.getState()._fetchGen + 1,
    })
    setSort({ field: 'updated_at', order: 'desc' })
    useTitleStore.getState().fetchTitles()
  }, [setSort])

  const hideNavbar = currentPath.startsWith('/login') || currentPath.startsWith('/wrapped')
  const pathname = currentPath.split('?')[0]
  const isSearch = pathname === '/search'
  const showDrawer = pathname === '/' || isSearch
  const mergeSourceId = isSearch
    ? new URLSearchParams(currentPath.split('?')[1] ?? '').get('mergeSourceId')
    : null

  let activeFilterCount = 0
  if (statusFilter !== null) activeFilterCount++
  if (typeFilter !== null) activeFilterCount++
  if (filter.is_anime === 'true') activeFilterCount++
  if (typeFilter === 'series' && seriesStatusFilter !== null) activeFilterCount++
  if (filter.decade || filter.release_from || filter.release_to) activeFilterCount++
  if (filter.genres && filter.genres.length > 0) activeFilterCount++
  if (filter.origin_country && filter.origin_country.length > 0) activeFilterCount++
  if (filter.my_rating_min) activeFilterCount++
  if (filter.tmdb_rating_min) activeFilterCount++
  if (!isSearch && (sort.field !== 'updated_at' || sort.order !== 'desc') && sort.field !== 'release_date') {
    activeFilterCount++
  }

  const drawerFilter: FilterState = {
    status: statusFilter,
    type: typeFilter,
    isAnime: filter.is_anime === 'true',
    seriesStatus: seriesStatusFilter,
    decade: filter.decade ?? null,
    releaseFrom: filter.release_from ?? '',
    releaseTo: filter.release_to ?? '',
    includeNoRelease: filter.include_no_release !== 'false',
    selectedGenres: filter.genres ?? [],
    genreOp: filter.genre_op ?? 'OR',
    selectedCountries: filter.origin_country ?? [],
    myRatingMin: filter.my_rating_min ?? '',
    tmdbRatingMin: filter.tmdb_rating_min ?? '',
  }

  const drawerActions: FilterActions = {
    onStatusChange: handleStatusChange,
    onTypeChange: handleTypeChange,
    onIsAnimeChange: handleIsAnimeChange,
    onSeriesStatusChange: handleSeriesStatusChange,
    onDecadeChange: handleDecadeChange,
    onReleaseFromChange: handleReleaseFromChange,
    onReleaseToChange: handleReleaseToChange,
    onIncludeNoReleaseChange: handleIncludeNoReleaseChange,
    onGenreToggle: handleGenreToggle,
    onGenreOpChange: handleGenreOpChange,
    onCountryToggle: handleCountryToggle,
    onMyRatingMinChange: handleMyRatingMinChange,
    onTmdbRatingMinChange: handleTmdbRatingMinChange,
  }

  const filterDrawer = showDrawer ? (
    <FilterDrawer
      filter={drawerFilter}
      actions={drawerActions}
      sort={sort}
      onSortChange={setSort}
      isSearchActive={isSearch}
      open={filterDrawerOpen}
      onOpenChange={setFilterDrawerOpen}
      onReset={handleResetFilters}
      activeCount={activeFilterCount}
      defaultOpen={false}
    />
  ) : null

  const isDesktop = useIsDesktop()
  const playback = useActivePlayback()
  const [playbackDrawerOpen, setPlaybackDrawerOpen] = useState(false)

  const handleToggleFilterDrawer = useCallback(() => {
    setFilterDrawerOpen((prev) => !prev)
  }, [])

  const liveIndicator = playback.session ? (
    <LivePlaybackIndicator
      session={playback.session}
      onClick={() => setPlaybackDrawerOpen(true)}
    />
  ) : null

  const above = (
    <>
      {liveIndicator}
      {isSearch
        ? (isDesktop ? null : (
            <>
              {filterDrawer}
              <SearchBar
                showTMDBToggle={!mergeSourceId}
                isFiltersOpen={filterDrawerOpen}
                onToggleFilters={handleToggleFilterDrawer}
                activeFilterCount={activeFilterCount}
              />
            </>
          ))
        : filterDrawer}
    </>
  )

  return (
    <ErrorBoundary>
      <UndoProvider>
        <div className={clsx(s.root, sidebarCollapsed && s.rootSidebarCollapsed, !hideNavbar && s.withNavbar, isSearch && s.withSearchBar)}>
          {!hideNavbar && (
            <Sidebar
              currentPath={currentPath}
              onNavigate={navigate}
              collapsed={sidebarCollapsed}
              onToggleCollapse={handleToggleSidebar}
            />
          )}
          <main className={s.main} id="main-content">
            <Suspense fallback={null}>
              <Router onChange={handleRoute}>
                <Library path={ROUTE_PATHS.home} filterOpen={filterDrawerOpen} />
                <Calendar path={ROUTE_PATHS.calendar} />
                <ComingUp path={ROUTE_PATHS.comingUp} />
                <ContinueWatching path={ROUTE_PATHS.continueWatching} />
                <Releases path={ROUTE_PATHS.releases} />
                <Search
                  path={ROUTE_PATHS.search}
                  filterOpen={filterDrawerOpen}
                  onToggleFilters={handleToggleFilterDrawer}
                  desktopFilterDrawer={isDesktop ? filterDrawer : undefined}
                  activeFilterCount={activeFilterCount}
                />
                <Add path={ROUTE_PATHS.add} />
                <Stats path={ROUTE_PATHS.stats} />
                <Wrapped path={ROUTE_PATHS.wrapped} />
                <Wrapped path={ROUTE_PATHS.wrappedYear} />
                <Login path={ROUTE_PATHS.login} />
                <Setup path={ROUTE_PATHS.setup} />
                <TitleDetail path={ROUTE_PATHS.title} />
                <PersonTitles path={ROUTE_PATHS.person} />
                <Admin path={ROUTE_PATHS.admin} />
                <AdminSettings path={ROUTE_PATHS.adminSettings} />
                <AdminAuth path={ROUTE_PATHS.adminAuth} />
                <Validate path={ROUTE_PATHS.adminValidate} />
                <AdminTasks path={ROUTE_PATHS.adminTasks} />
                <AdminNotifications path={ROUTE_PATHS.adminNotifications} />
                <AdminJellyfin path={ROUTE_PATHS.adminJellyfin} />
                <AdminAniList path={ROUTE_PATHS.adminAniList} />
                <AdminArr path={ROUTE_PATHS.adminArr} />
                <AdminSeasonAudit path={ROUTE_PATHS.adminSeasonAudit} />
                <Help path={ROUTE_PATHS.adminHelp} />
                <AnilistCallback path={ROUTE_PATHS.anilistCallback} />
                <MatchReview path={ROUTE_PATHS.matchReview} />
              </Router>
            </Suspense>
          </main>
          <UndoSnackbar hideNavbar={hideNavbar} />
          <LivePlaybackDrawer
            session={playback.rawSession}
            open={playbackDrawerOpen}
            onClose={() => setPlaybackDrawerOpen(false)}
            onDismiss={() => {
              playback.dismiss()
              setPlaybackDrawerOpen(false)
            }}
            onWatchedSuccess={() => {
              playback.refresh()
            }}
          />
          {!hideNavbar && (
            <Navbar
              currentPath={currentPath}
              onNavigate={navigate}
              above={above}
            />
          )}
        </div>
      </UndoProvider>
    </ErrorBoundary>
  )
}
