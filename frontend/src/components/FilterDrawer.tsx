import { useState, useEffect, useRef, useCallback } from 'preact/hooks'
import clsx from 'clsx'
import type { GenreCount, CountryCount } from '../types'
import { apiFetch } from '../api'
import { countryLabel } from '../lib/country'
import { useTranslation } from '../i18n'
import { useSwipeDownToClose } from '../hooks/useSwipeDownToClose'
import type {
  StatusFilter,
  TypeFilter,
  SeriesStatusFilter,
  DrawerTab,
  FilterState,
  FilterActions,
  FilterDrawerProps,
} from './filter/types'
import {
  statusFilters,
  typeFilters,
  sortOptions,
  seriesStatusFilters,
  decadeOptions,
} from './filter/types'
import { FilterBasicsTab } from './filter/FilterBasicsTab'
import { FilterGenresTab } from './filter/FilterGenresTab'
import { FilterDatesTab } from './filter/FilterDatesTab'
import s from './FilterDrawer.module.css'

export type {
  StatusFilter,
  TypeFilter,
  SeriesStatusFilter,
  DrawerTab,
  FilterState,
  FilterActions,
  FilterDrawerProps,
}

const STORAGE_KEY_HOME = 'filter-drawer-open-home-v2'

function resolveFilterValues(props: FilterDrawerProps): { filter: FilterState; actions: FilterActions } {
  if (props.filter && props.actions) {
    return { filter: props.filter, actions: props.actions }
  }
  const filter: FilterState = {
    status: props.status ?? null,
    type: props.type ?? null,
    isAnime: Boolean(props.isAnime),
    seriesStatus: props.seriesStatus ?? null,
    decade: props.decade ?? null,
    releaseFrom: props.releaseFrom ?? '',
    releaseTo: props.releaseTo ?? '',
    includeNoRelease: props.includeNoRelease ?? true,
    selectedGenres: props.selectedGenres ?? [],
    genreOp: props.genreOp ?? 'OR',
    selectedCountries: props.selectedCountries ?? [],
    myRatingMin: props.myRatingMin ?? '',
    tmdbRatingMin: props.tmdbRatingMin ?? '',
  }
  const actions: FilterActions = {
    onStatusChange: props.onStatusChange ?? (() => {}),
    onTypeChange: props.onTypeChange ?? (() => {}),
    onIsAnimeChange: props.onIsAnimeChange ?? (() => {}),
    onSeriesStatusChange: props.onSeriesStatusChange ?? (() => {}),
    onDecadeChange: props.onDecadeChange ?? (() => {}),
    onReleaseFromChange: props.onReleaseFromChange ?? (() => {}),
    onReleaseToChange: props.onReleaseToChange ?? (() => {}),
    onIncludeNoReleaseChange: props.onIncludeNoReleaseChange ?? (() => {}),
    onGenreToggle: props.onGenreToggle ?? (() => {}),
    onGenreOpChange: props.onGenreOpChange ?? (() => {}),
    onCountryToggle: props.onCountryToggle ?? (() => {}),
    onMyRatingMinChange: props.onMyRatingMinChange ?? (() => {}),
    onTmdbRatingMinChange: props.onTmdbRatingMinChange ?? (() => {}),
  }
  return { filter, actions }
}

export function FilterDrawer(props: FilterDrawerProps) {
  const {
    sort,
    onSortChange,
    isSearchActive,
    open: openProp,
    onOpenChange,
    onReset,
    activeCount,
    defaultOpen = false,
  } = props

  const { filter, actions } = resolveFilterValues(props)
  const { t } = useTranslation()

  const [internalOpen, setInternalOpen] = useState(() => {
    const stored = localStorage.getItem(STORAGE_KEY_HOME)
    return stored !== null ? stored === 'true' : defaultOpen
  })
  const isControlled = openProp !== undefined
  const open = isControlled ? openProp : internalOpen

  const setOpen = useCallback((next: boolean | ((prev: boolean) => boolean)) => {
    const nextVal = typeof next === 'function' ? next(open) : next
    if (!isControlled) {
      setInternalOpen(nextVal)
    }
    onOpenChange?.(nextVal)
    localStorage.setItem(STORAGE_KEY_HOME, String(nextVal))
  }, [isControlled, open, onOpenChange])

  const [activeTab, setActiveTab] = useState<DrawerTab>('basics')
  const genreDropdownRef = useRef<HTMLDivElement>(null)
  const countryDropdownRef = useRef<HTMLDivElement>(null)
  const [genres, setGenres] = useState<GenreCount[]>([])
  const [countries, setCountries] = useState<CountryCount[]>([])

  const { ref: containerRef, dragY, style: swipeStyle } = useSwipeDownToClose({
    open,
    onClose: () => setOpen(false),
    shouldIgnore: (target) => {
      const node = target as Node | null
      return Boolean(node && (genreDropdownRef.current?.contains(node) || countryDropdownRef.current?.contains(node)))
    },
  })

  useEffect(() => {
    apiFetch<CountryCount[]>('/countries').then(setCountries).catch(() => { /* ignore */ })
  }, [])

  const fetchGenres = useCallback(() => {
    apiFetch<GenreCount[]>('/genres')
      .then(setGenres)
      .catch(() => { /* ignore */ })
  }, [])

  useEffect(() => {
    fetchGenres()
  }, [fetchGenres])

  useEffect(() => {
    localStorage.setItem(STORAGE_KEY_HOME, String(open))
  }, [open])

  const showSeriesStatus = filter.type === 'series'

  // Build active tags for collapsed state
  const activeTags: string[] = []
  const activeSort = sortOptions.find((o) => o.field === sort.field)
  if (!isSearchActive && activeSort && sort.field !== 'release_date') {
    activeTags.push(`${activeSort.label} ${sort.order === 'asc' ? '↑' : '↓'}`)
  }
  const activeStatus = statusFilters.find((f) => f.id === filter.status)
  if (filter.status !== null && activeStatus) activeTags.push(activeStatus.label)
  if (filter.isAnime) activeTags.push('Anime')
  const activeType = typeFilters.find((f) => f.id === filter.type)
  if (filter.type !== null && activeType) activeTags.push(activeType.label)
  if (showSeriesStatus && filter.seriesStatus !== null) {
    const activeSeries = seriesStatusFilters.find((f) => f.id === filter.seriesStatus)
    if (activeSeries) activeTags.push(activeSeries.label)
  }
  if (filter.decade) {
    activeTags.push(decadeOptions.find((o) => o.value === filter.decade)?.label ?? filter.decade)
  } else if (filter.releaseFrom || filter.releaseTo) {
    activeTags.push(
      filter.releaseFrom && filter.releaseTo
        ? `${filter.releaseFrom.slice(0, 7)} → ${filter.releaseTo.slice(0, 7)}`
        : filter.releaseFrom ? `≥ ${filter.releaseFrom}` : `≤ ${filter.releaseTo}`,
    )
  }
  if (filter.selectedGenres.length > 0) {
    activeTags.push(`${filter.selectedGenres.length} genre${filter.selectedGenres.length > 1 ? 's' : ''}`)
  }
  if (filter.selectedCountries.length > 0) {
    activeTags.push(filter.selectedCountries.map(countryLabel).join(', '))
  }
  if (filter.myRatingMin) activeTags.push(`My ★≥${filter.myRatingMin}`)
  if (filter.tmdbRatingMin) activeTags.push(`TMDB≥${filter.tmdbRatingMin}`)

  const hasBasicsActive = Boolean(
    filter.status !== null ||
    filter.type !== null ||
    filter.isAnime ||
    (showSeriesStatus && filter.seriesStatus !== null) ||
    (!isSearchActive && (sort.field !== 'updated_at' || sort.order !== 'desc'))
  )
  const hasGenresActive = filter.selectedGenres.length > 0 || filter.selectedCountries.length > 0
  const hasDatesActive = Boolean(
    filter.decade ||
    filter.releaseFrom ||
    filter.releaseTo ||
    !filter.includeNoRelease ||
    filter.myRatingMin ||
    filter.tmdbRatingMin
  )

  const activeFilterCount = activeCount ?? activeTags.length

  return (
    <div
      ref={containerRef}
      style={swipeStyle}
      className={s.container}
    >
      {/* Handle */}
      {(!isSearchActive || open) && (
        <div className={s.handle} onClick={() => setOpen(!open)}>
          <div className={s.handleTop}>
            <div className={s.handleBar} />
            <span className={s.handleText}>{t('search.filters')}</span>
            <div className={s.handleBar} />
          </div>
          {!open && activeTags.length > 0 && (
            <div className={s.activeTags}>
              {activeTags.map((tag) => (
                <span key={tag} className={s.activeTag}>{tag}</span>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Drawer content */}
      <div className={clsx(s.drawer, open ? s.drawerExpanded : s.drawerCollapsed)}>
        {/* Drawer Header */}
        <div className={s.drawerHeader}>
          <span className={s.drawerHeaderTitle}>
            {activeFilterCount > 0
              ? t('search.filtersActive', { count: activeFilterCount })
              : t('search.filtersTitle')}
          </span>
          {activeFilterCount > 0 && onReset && (
            <button
              type="button"
              className={s.resetFiltersBtn}
              onClick={onReset}
              title={t('search.resetFilters')}
            >
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/>
                <path d="M3 3v5h5"/>
              </svg>
              <span>{t('search.resetFilters')}</span>
            </button>
          )}
        </div>

        {/* Sub-tab Navigation */}
        <div className={s.tabBar} role="tablist">
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'basics'}
            className={clsx(s.tabBtn, activeTab === 'basics' && s.tabBtnActive)}
            onClick={() => setActiveTab('basics')}
          >
            <span>Status & Type</span>
            {hasBasicsActive && <span className={s.tabDot} />}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'genres'}
            className={clsx(s.tabBtn, activeTab === 'genres' && s.tabBtnActive)}
            onClick={() => setActiveTab('genres')}
          >
            <span>Genres & Origin</span>
            {hasGenresActive && <span className={s.tabDot} />}
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={activeTab === 'dates'}
            className={clsx(s.tabBtn, activeTab === 'dates' && s.tabBtnActive)}
            onClick={() => setActiveTab('dates')}
          >
            <span>Dates & Ratings</span>
            {hasDatesActive && <span className={s.tabDot} />}
          </button>
        </div>

        <div className={s.tabContent}>
          {activeTab === 'basics' && (
            <FilterBasicsTab
              filter={filter}
              actions={actions}
              sort={sort}
              onSortChange={onSortChange}
              isSearchActive={isSearchActive}
            />
          )}

          {activeTab === 'genres' && (
            <FilterGenresTab
              filter={filter}
              actions={actions}
              genres={genres}
              countries={countries}
              genreDropdownRef={genreDropdownRef}
              countryDropdownRef={countryDropdownRef}
            />
          )}

          {activeTab === 'dates' && (
            <FilterDatesTab
              filter={filter}
              actions={actions}
            />
          )}
        </div>

        <div className={s.bottomPad} />
      </div>
    </div>
  )
}
