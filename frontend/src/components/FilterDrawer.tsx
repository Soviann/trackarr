import { useState, useEffect, useRef, useCallback } from 'preact/hooks'
import clsx from 'clsx'
import type { TitleStatus, TitleType, SeriesStatus, GenreCount, CountryCount } from '../types'
import type { SortField, SortOrder, SortState } from '../store'
import { apiFetch } from '../api'
import { countryLabel, isRealCountry } from '../lib/country'
import s from './FilterDrawer.module.css'

const STORAGE_KEY_HOME = 'filter-drawer-open-home'

type StatusFilter = TitleStatus | 'up_to_date' | null
type TypeFilter = TitleType | null
type SeriesStatusFilter = SeriesStatus | null

interface FilterDrawerProps {
  status: StatusFilter
  type: TypeFilter
  isAnime: boolean
  seriesStatus: SeriesStatusFilter
  onStatusChange: (status: StatusFilter) => void
  onTypeChange: (type: TypeFilter) => void
  onIsAnimeChange: (isAnime: boolean) => void
  onSeriesStatusChange: (seriesStatus: SeriesStatusFilter) => void
  sort: SortState
  onSortChange: (sort: SortState) => void
  isSearchActive: boolean
  defaultOpen?: boolean
  decade: string | null
  releaseFrom: string
  releaseTo: string
  includeNoRelease: boolean
  onDecadeChange: (decade: string | null) => void
  onReleaseFromChange: (date: string) => void
  onReleaseToChange: (date: string) => void
  onIncludeNoReleaseChange: (include: boolean) => void
  selectedGenres: string[]
  genreOp: 'AND' | 'OR'
  onGenreToggle: (genre: string) => void
  onGenreOpChange: (op: 'AND' | 'OR') => void
  selectedCountries: string[]
  onCountryToggle: (iso: string) => void
  myRatingMin: string
  tmdbRatingMin: string
  onMyRatingMinChange: (v: string) => void
  onTmdbRatingMinChange: (v: string) => void
}

const statusFilters: { id: StatusFilter; label: string }[] = [
  { id: null, label: 'All' },
  { id: 'plan_to_watch', label: 'Plan' },
  { id: 'watching', label: 'Watching' },
  { id: 'up_to_date', label: 'Caught up' },
  { id: 'completed', label: 'Completed' },
  { id: 'dropped', label: 'Dropped' },
]

const typeFilters: { id: TypeFilter; label: string }[] = [
  { id: null, label: 'All' },
  { id: 'movie', label: 'Movie' },
  { id: 'series', label: 'Series' },
]

const sortOptions: { field: SortField; label: string; defaultOrder: SortOrder }[] = [
  { field: 'updated_at', label: 'Last updated', defaultOrder: 'desc' },
  { field: 'original_title', label: 'Title', defaultOrder: 'asc' },
  { field: 'release_date', label: 'Release date', defaultOrder: 'desc' },
  { field: 'my_rating', label: 'Rating', defaultOrder: 'desc' },
  { field: 'created_at', label: 'Date added', defaultOrder: 'desc' },
  { field: 'last_watched_at', label: 'Last watched', defaultOrder: 'desc' },
]

const seriesStatusFilters: { id: SeriesStatusFilter; label: string }[] = [
  { id: null, label: 'All' },
  { id: 'returning', label: 'Returning' },
  { id: 'ended', label: 'Ended' },
  { id: 'cancelled', label: 'Cancelled' },
  { id: 'in_production', label: 'Not started' },
]

const decadeOptions = [
  { value: '', label: 'Decade: All' },
  { value: '2020', label: '2020s' },
  { value: '2010', label: '2010s' },
  { value: '2000', label: '2000s' },
  { value: '1990', label: '1990s' },
  { value: '1980', label: '1980s' },
  { value: '1970', label: '1970s' },
  { value: '1960', label: '1960s' },
  { value: '1950', label: '1950s' },
  { value: '1940', label: '1940s' },
  { value: '1930', label: '1930s' },
  { value: '1920', label: '1920s' },
]

function Chip<T>({ filter, active, onClick }: {
  filter: { id: T; label: string }
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      className={clsx(s.chip, active && s.chipActive)}
      onClick={onClick}
    >
      {filter.label}
    </button>
  )
}

export function FilterDrawer({
  status, type, isAnime, seriesStatus,
  onStatusChange, onTypeChange, onIsAnimeChange, onSeriesStatusChange,
  sort, onSortChange, isSearchActive,
  defaultOpen = true,
  decade, releaseFrom, releaseTo, includeNoRelease,
  onDecadeChange, onReleaseFromChange, onReleaseToChange, onIncludeNoReleaseChange,
  selectedGenres, genreOp, onGenreToggle, onGenreOpChange,
  selectedCountries, onCountryToggle, myRatingMin, tmdbRatingMin,
  onMyRatingMinChange, onTmdbRatingMinChange,
}: FilterDrawerProps) {
  const [open, setOpen] = useState(() => {
    if (!defaultOpen) return false
    const stored = localStorage.getItem(STORAGE_KEY_HOME)
    return stored !== null ? stored === 'true' : true
  })
  const [dragY, setDragY] = useState(0)
  const touchStartY = useRef<number | null>(null)
  const dragYRef = useRef(0)
  const openRef = useRef(open)
  const containerRef = useRef<HTMLDivElement>(null)
  const genreDropdownRef = useRef<HTMLDivElement>(null)
  const [genres, setGenres] = useState<GenreCount[]>([])
  const [genreSearch, setGenreSearch] = useState('')
  const [genreDropdownOpen, setGenreDropdownOpen] = useState(false)
  const genreBlurTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const genreInputRef = useRef<HTMLInputElement>(null)
  const [countries, setCountries] = useState<CountryCount[]>([])
  const [countrySearch, setCountrySearch] = useState('')
  const [countryDropdownOpen, setCountryDropdownOpen] = useState(false)
  const countryBlurTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const countryInputRef = useRef<HTMLInputElement>(null)
  const countryDropdownRef = useRef<HTMLDivElement>(null)
  useEffect(() => {
    apiFetch<CountryCount[]>('/countries').then(setCountries).catch(() => { /* ignore */ })
  }, [])

  useEffect(() => { openRef.current = open }, [open])
  useEffect(() => { dragYRef.current = dragY }, [dragY])

  const fetchGenres = useCallback(() => {
    apiFetch<GenreCount[]>('/genres')
      .then(setGenres)
      .catch(() => { /* ignore */ })
  }, [])

  useEffect(() => {
    fetchGenres()
  }, [fetchGenres])

  const handleTouchStart = useCallback((e: TouchEvent) => {
    if (!openRef.current) return
    // Skip drag tracking when starting inside a scrollable autocomplete dropdown
    const target = e.target as Node | null
    if (target && (genreDropdownRef.current?.contains(target) || countryDropdownRef.current?.contains(target))) return
    touchStartY.current = e.touches[0].clientY
  }, [])

  const handleTouchMove = useCallback((e: TouchEvent) => {
    if (touchStartY.current === null) return
    const deltaY = e.touches[0].clientY - touchStartY.current
    if (deltaY > 0) {
      // Block native page scroll while we're handling the close gesture
      e.preventDefault()
      setDragY(deltaY)
    }
  }, [])

  const handleTouchEnd = useCallback(() => {
    if (touchStartY.current === null) return
    if (dragYRef.current > 100) setOpen(false)
    setDragY(0)
    touchStartY.current = null
  }, [])

  // Attach touch listeners with { passive: false } so preventDefault can stop page scroll
  useEffect(() => {
    const el = containerRef.current
    if (!el) return
    el.addEventListener('touchstart', handleTouchStart, { passive: true })
    el.addEventListener('touchmove', handleTouchMove, { passive: false })
    el.addEventListener('touchend', handleTouchEnd, { passive: true })
    el.addEventListener('touchcancel', handleTouchEnd, { passive: true })
    return () => {
      el.removeEventListener('touchstart', handleTouchStart)
      el.removeEventListener('touchmove', handleTouchMove)
      el.removeEventListener('touchend', handleTouchEnd)
      el.removeEventListener('touchcancel', handleTouchEnd)
    }
  }, [handleTouchStart, handleTouchMove, handleTouchEnd])

  // Reset to closed when switching to a page with defaultOpen=false
  useEffect(() => {
    if (!defaultOpen) setOpen(false)
  }, [defaultOpen])

  useEffect(() => {
    if (defaultOpen) localStorage.setItem(STORAGE_KEY_HOME, String(open))
  }, [open, defaultOpen])

  const showSeriesStatus = type === 'series'

  const handleSortClick = (option: typeof sortOptions[number]) => {
    if (sort.field === option.field) {
      onSortChange({ field: sort.field, order: sort.order === 'asc' ? 'desc' : 'asc' })
    } else {
      onSortChange({ field: option.field, order: option.defaultOrder })
    }
  }

  // Build active tags for collapsed state — single tan accent across all tags
  const activeTags: string[] = []
  const activeSort = sortOptions.find((o) => o.field === sort.field)
  if (!isSearchActive && activeSort && sort.field !== 'release_date') {
    activeTags.push(`${activeSort.label} ${sort.order === 'asc' ? '↑' : '↓'}`)
  }
  const activeStatus = statusFilters.find((f) => f.id === status)
  if (status !== null && activeStatus) activeTags.push(activeStatus.label)
  if (isAnime) activeTags.push('Anime')
  const activeType = typeFilters.find((f) => f.id === type)
  if (type !== null && activeType) activeTags.push(activeType.label)
  if (showSeriesStatus && seriesStatus !== null) {
    const activeSeries = seriesStatusFilters.find((f) => f.id === seriesStatus)
    if (activeSeries) activeTags.push(activeSeries.label)
  }
  if (decade) {
    activeTags.push(decadeOptions.find((o) => o.value === decade)?.label ?? decade)
  } else if (releaseFrom || releaseTo) {
    activeTags.push(
      releaseFrom && releaseTo
        ? `${releaseFrom.slice(0, 7)} → ${releaseTo.slice(0, 7)}`
        : releaseFrom ? `≥ ${releaseFrom}` : `≤ ${releaseTo}`,
    )
  }
  if (selectedGenres.length > 0) {
    activeTags.push(`${selectedGenres.length} genre${selectedGenres.length > 1 ? 's' : ''}`)
  }
  if (selectedCountries.length > 0) {
    activeTags.push(selectedCountries.map(countryLabel).join(', '))
  }
  if (myRatingMin) activeTags.push(`My ★≥${myRatingMin}`)
  if (tmdbRatingMin) activeTags.push(`TMDB≥${tmdbRatingMin}`)

  return (
    <div
      ref={containerRef}
      style={dragY > 0 ? { transform: `translateY(${dragY}px)`, transition: 'none' } : undefined}
      className={s.container}
    >
      {/* Handle */}
      <div className={s.handle} onClick={() => setOpen(!open)}>
        <div className={s.handleTop}>
          <div className={s.handleBar} />
          <span className={s.handleText}>Filters</span>
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

      {/* Drawer content */}
      <div className={clsx(s.drawer, open ? s.drawerExpanded : s.drawerCollapsed)}>
        {!isSearchActive && (
          <>
            <div className={clsx(s.filterLabel, s.filterLabelFirst)}>Sort</div>
            <div className={s.filterRow}>
              {sortOptions.map((opt) => {
                const active = sort.field === opt.field
                return (
                  <button
                    key={opt.field}
                    className={clsx(s.chip, active && s.chipActive)}
                    onClick={() => handleSortClick(opt)}
                  >
                    {opt.label}
                    {active && (
                      <span className={s.sortArrow}>
                        {sort.order === 'asc' ? ' ↑' : ' ↓'}
                      </span>
                    )}
                  </button>
                )
              })}
            </div>
          </>
        )}
        <div className={clsx(s.filterLabel, isSearchActive && s.filterLabelFirst)}>Status</div>
        <div className={s.filterRow}>
          {statusFilters.map((f) => (
            <Chip key={f.label} filter={f} active={status === f.id} onClick={() => onStatusChange(f.id)} />
          ))}
        </div>

        <div className={s.filterLabel}>Type</div>
        <div className={s.filterRow}>
          {typeFilters.map((f) => (
            <Chip key={f.label} filter={f} active={type === f.id} onClick={() => onTypeChange(f.id)} />
          ))}
          <Chip
            filter={{ id: true, label: 'Anime' }}
            active={isAnime}
            onClick={() => onIsAnimeChange(!isAnime)}
          />
        </div>

        {showSeriesStatus && (
          <>
            <div className={s.filterLabel}>Series status</div>
            <div className={s.filterRow}>
              {seriesStatusFilters.map((f) => (
                <Chip
                  key={f.label}
                  filter={f}
                  active={seriesStatus === f.id}
                  onClick={() => onSeriesStatusChange(f.id)}
                />
              ))}
            </div>
          </>
        )}

        {genres.length > 0 && (() => {
          const sortedGenres = [...genres].sort((a, b) => a.genre.localeCompare(b.genre))
          const filteredGenres = sortedGenres
            .filter(g => !selectedGenres.includes(g.genre))
            .filter(g => !genreSearch || g.genre.toLowerCase().includes(genreSearch.toLowerCase()))
          return (
            <>
              <div className={s.filterLabel}>Genres</div>
              <div className={s.genreOpRow}>
                <button
                  className={clsx(s.opBtn, genreOp === 'OR' && s.opBtnActive)}
                  onClick={() => onGenreOpChange('OR')}
                >Any</button>
                <button
                  className={clsx(s.opBtn, genreOp === 'AND' && s.opBtnActive)}
                  onClick={() => onGenreOpChange('AND')}
                >All</button>
                {selectedGenres.length > 0 && (
                  <button
                    className={s.clearGenresBtn}
                    onClick={() => selectedGenres.forEach(g => onGenreToggle(g))}
                  >Clear</button>
                )}
              </div>
              <div className={s.genreDropdownWrapper}>
                <div
                  className={s.genreAutocomplete}
                  onClick={() => genreInputRef.current?.focus()}
                >
                  {selectedGenres.map(g => (
                    <span key={g} className={s.genreTag}>
                      {g}
                      <button
                        className={s.genreTagRemove}
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={(e) => { e.stopPropagation(); onGenreToggle(g) }}
                      >&times;</button>
                    </span>
                  ))}
                  <input
                    ref={genreInputRef}
                    type="text"
                    className={s.genreInput}
                    placeholder={selectedGenres.length === 0 ? 'Search genres…' : ''}
                    value={genreSearch}
                    onInput={(e) => setGenreSearch((e.target as HTMLInputElement).value)}
                    onFocus={() => {
                      if (genreBlurTimeout.current) clearTimeout(genreBlurTimeout.current)
                      setGenreDropdownOpen(true)
                    }}
                    onBlur={() => {
                      genreBlurTimeout.current = setTimeout(() => setGenreDropdownOpen(false), 150)
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Escape') {
                        setGenreDropdownOpen(false)
                        ;(e.target as HTMLInputElement).blur()
                      }
                    }}
                  />
                </div>
                {genreDropdownOpen && filteredGenres.length > 0 && (
                  <div ref={genreDropdownRef} className={s.genreDropdown}>
                    {filteredGenres.map(g => (
                      <div
                        key={g.genre}
                        className={s.genreDropdownItem}
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => {
                          onGenreToggle(g.genre)
                          setGenreSearch('')
                          genreInputRef.current?.focus()
                        }}
                      >
                        <span>{g.genre}</span>
                        <span className={s.genreDropdownCount}>{g.count}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </>
          )
        })()}

        {countries.some(c => isRealCountry(c.country)) && (() => {
          const q = countrySearch.toLowerCase()
          const filteredCountries = countries
            .filter(c => isRealCountry(c.country))
            .filter(c => !selectedCountries.includes(c.country))
            .filter(c => !q || countryLabel(c.country).toLowerCase().includes(q) || c.country.toLowerCase().includes(q))
          return (
            <>
              <div className={s.filterLabel}>Country</div>
              {selectedCountries.length > 0 && (
                <div className={s.genreOpRow}>
                  <button
                    className={s.clearGenresBtn}
                    onClick={() => selectedCountries.forEach(c => onCountryToggle(c))}
                  >Clear</button>
                </div>
              )}
              <div className={s.genreDropdownWrapper}>
                <div
                  className={s.genreAutocomplete}
                  onClick={() => countryInputRef.current?.focus()}
                >
                  {selectedCountries.map(c => (
                    <span key={c} className={s.genreTag}>
                      {countryLabel(c)}
                      <button
                        className={s.genreTagRemove}
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={(e) => { e.stopPropagation(); onCountryToggle(c) }}
                      >&times;</button>
                    </span>
                  ))}
                  <input
                    ref={countryInputRef}
                    type="text"
                    className={s.genreInput}
                    placeholder={selectedCountries.length === 0 ? 'Search countries…' : ''}
                    value={countrySearch}
                    onInput={(e) => setCountrySearch((e.target as HTMLInputElement).value)}
                    onFocus={() => {
                      if (countryBlurTimeout.current) clearTimeout(countryBlurTimeout.current)
                      setCountryDropdownOpen(true)
                    }}
                    onBlur={() => {
                      countryBlurTimeout.current = setTimeout(() => setCountryDropdownOpen(false), 150)
                    }}
                    onKeyDown={(e) => {
                      if (e.key === 'Escape') {
                        setCountryDropdownOpen(false)
                        ;(e.target as HTMLInputElement).blur()
                      }
                    }}
                  />
                </div>
                {countryDropdownOpen && filteredCountries.length > 0 && (
                  <div ref={countryDropdownRef} className={clsx(s.genreDropdown, s.dropUp)}>
                    {filteredCountries.map(c => (
                      <div
                        key={c.country}
                        className={s.genreDropdownItem}
                        onMouseDown={(e) => e.preventDefault()}
                        onClick={() => {
                          onCountryToggle(c.country)
                          setCountrySearch('')
                          countryInputRef.current?.focus()
                        }}
                      >
                        <span>{countryLabel(c.country)}</span>
                        <span className={s.genreDropdownCount}>{c.count}</span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </>
          )
        })()}

        <div className={s.filterLabel}>Rating</div>
        <div className={s.filterRow}>
          <select
            className={s.select}
            value={myRatingMin}
            onChange={(e) => onMyRatingMinChange((e.target as HTMLSelectElement).value)}
          >
            <option value="">My rating: any</option>
            {[1,2,3,4,5,6,7,8,9,10].map(n => (
              <option key={n} value={String(n)}>My rating ≥ {n}</option>
            ))}
          </select>
          <select
            className={s.select}
            value={tmdbRatingMin}
            onChange={(e) => onTmdbRatingMinChange((e.target as HTMLSelectElement).value)}
          >
            <option value="">TMDB: any</option>
            {[5,6,7,8,9].map(n => (
              <option key={n} value={String(n)}>TMDB ≥ {n}</option>
            ))}
          </select>
        </div>

        <div className={s.filterLabel}>Release date</div>
        <div className={s.filterRow}>
          <select
            className={s.select}
            value={decade ?? ''}
            onChange={(e) => {
              const val = (e.target as HTMLSelectElement).value
              onDecadeChange(val || null)
              if (val) {
                onReleaseFromChange('')
                onReleaseToChange('')
              }
            }}
          >
            {decadeOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>{opt.label}</option>
            ))}
          </select>
          <input
            type="date"
            className={s.dateInput}
            value={releaseFrom}
            placeholder="From"
            onChange={(e) => {
              onReleaseFromChange((e.target as HTMLInputElement).value)
              if ((e.target as HTMLInputElement).value) onDecadeChange(null)
            }}
          />
          <input
            type="date"
            className={s.dateInput}
            value={releaseTo}
            placeholder="To"
            onChange={(e) => {
              onReleaseToChange((e.target as HTMLInputElement).value)
              if ((e.target as HTMLInputElement).value) onDecadeChange(null)
            }}
          />
        </div>
        {(decade || releaseFrom || releaseTo) && (
          <div className={s.filterRow}>
            <label className={s.toggleLabel}>
              <input
                type="checkbox"
                checked={includeNoRelease}
                onChange={(e) => onIncludeNoReleaseChange((e.target as HTMLInputElement).checked)}
              />
              <span>Include without release date</span>
            </label>
          </div>
        )}

        <div className={s.bottomPad} />
      </div>
    </div>
  )
}
