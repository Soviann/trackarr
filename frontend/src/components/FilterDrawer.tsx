import { useState, useEffect, useRef, useCallback } from 'preact/hooks'
import clsx from 'clsx'
import type { TitleStatus, TitleType, SeriesStatus, GenreCount } from '../types'
import type { SortField, SortOrder, SortState } from '../store'
import { apiFetch } from '../api'
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
  { id: 'in_production', label: 'In prod.' },
]

const decadeOptions = [
  { value: '', label: 'All' },
  { value: '2000', label: '2000s' },
  { value: '2010', label: '2010s' },
  { value: '2020', label: '2020s' },
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
}: FilterDrawerProps) {
  const [open, setOpen] = useState(() => {
    if (!defaultOpen) return false
    const stored = localStorage.getItem(STORAGE_KEY_HOME)
    return stored !== null ? stored === 'true' : true
  })
  const [dragY, setDragY] = useState(0)
  const touchStartY = useRef<number | null>(null)
  const [genres, setGenres] = useState<GenreCount[]>([])
  const [genreSearch, setGenreSearch] = useState('')
  const [genreDropdownOpen, setGenreDropdownOpen] = useState(false)
  const genreBlurTimeout = useRef<ReturnType<typeof setTimeout> | null>(null)
  const genreInputRef = useRef<HTMLInputElement>(null)

  const fetchGenres = useCallback(() => {
    apiFetch<GenreCount[]>('/genres')
      .then(setGenres)
      .catch(() => { /* ignore */ })
  }, [])

  useEffect(() => {
    fetchGenres()
  }, [fetchGenres])

  const handleTouchStart = (e: TouchEvent) => {
    if (!open) return
    touchStartY.current = e.touches[0].clientY
  }

  const handleTouchMove = (e: TouchEvent) => {
    if (!open || touchStartY.current === null) return
    const deltaY = e.touches[0].clientY - touchStartY.current
    if (deltaY > 0) {
      setDragY(deltaY)
    }
  }

  const handleTouchEnd = () => {
    if (!open || touchStartY.current === null) return
    if (dragY > 100) {
      setOpen(false)
    }
    setDragY(0)
    touchStartY.current = null
  }

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

  return (
    <div
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
      style={dragY > 0 ? { transform: `translateY(${dragY}px)`, transition: 'none' } : undefined}
      className={s.container}
    >
      {/* Handle */}
      <div className={s.handle} onClick={() => setOpen(!open)}>
        <div className={s.handleBar} />
        <span className={s.handleText}>Filters</span>
        <span className={clsx(s.chevron, open && s.chevronOpen)}>&#9650;</span>
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
                  <div className={s.genreDropdown}>
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
