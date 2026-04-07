import { useState, useEffect, useRef } from 'preact/hooks'
import clsx from 'clsx'
import type { TitleStatus, TitleType, SeriesStatus } from '../types'
import type { SortField, SortOrder, SortState } from '../store'
import { colors, accentWash } from '../theme'
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
}

const statusFilters: { id: StatusFilter; label: string; color: string }[] = [
  { id: null, label: 'All', color: colors.accentTeal },
  { id: 'plan_to_watch', label: 'Plan', color: colors.accentLavender },
  { id: 'watching', label: 'Watching', color: colors.accentAmber },
  { id: 'up_to_date', label: 'Caught up', color: colors.accentBlue },
  { id: 'completed', label: 'Completed', color: colors.accentGreen },
  { id: 'dropped', label: 'Dropped', color: colors.accentCoral },
]

const typeFilters: { id: TypeFilter; label: string; color: string }[] = [
  { id: null, label: 'All', color: colors.accentTeal },
  { id: 'movie', label: 'Movie', color: colors.accentAmber },
  { id: 'series', label: 'Series', color: colors.accentLavender },
]

const sortOptions: { field: SortField; label: string; defaultOrder: SortOrder }[] = [
  { field: 'updated_at', label: 'Last updated', defaultOrder: 'desc' },
  { field: 'original_title', label: 'Title', defaultOrder: 'asc' },
  { field: 'release_date', label: 'Release date', defaultOrder: 'desc' },
  { field: 'my_rating', label: 'Rating', defaultOrder: 'desc' },
  { field: 'created_at', label: 'Date added', defaultOrder: 'desc' },
  { field: 'last_watched_at', label: 'Last watched', defaultOrder: 'desc' },
]

const seriesStatusFilters: { id: SeriesStatusFilter; label: string; color: string }[] = [
  { id: null, label: 'All', color: colors.accentTeal },
  { id: 'returning', label: 'Returning', color: colors.accentGreen },
  { id: 'ended', label: 'Ended', color: colors.textSecondary },
  { id: 'cancelled', label: 'Cancelled', color: colors.accentCoral },
  { id: 'in_production', label: 'In prod.', color: colors.accentTeal },
]

const decadeOptions = [
  { value: '', label: 'All' },
  { value: '2000', label: '2000s' },
  { value: '2010', label: '2010s' },
  { value: '2020', label: '2020s' },
]

function Chip<T>({ filter, active, onClick }: {
  filter: { id: T; label: string; color: string }
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      className={clsx(s.chip, active && s.chipActive)}
      style={active ? { background: accentWash(filter.color), color: filter.color } : undefined}
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
}: FilterDrawerProps) {
  const [open, setOpen] = useState(() => {
    if (!defaultOpen) return false
    const stored = localStorage.getItem(STORAGE_KEY_HOME)
    return stored !== null ? stored === 'true' : true
  })
  const [dragY, setDragY] = useState(0)
  const touchStartY = useRef<number | null>(null)

  const handleTouchStart = (e: any) => {
    if (!open) return
    touchStartY.current = e.touches[0].clientY
  }

  const handleTouchMove = (e: any) => {
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

  // Build active tags for collapsed state
  const activeTags: { label: string; color: string }[] = []
  const activeSort = sortOptions.find((o) => o.field === sort.field)
  if (!isSearchActive && activeSort && sort.field !== 'release_date') {
    activeTags.push({ label: `${activeSort.label} ${sort.order === 'asc' ? '↑' : '↓'}`, color: colors.accentTeal })
  }
  const activeStatus = statusFilters.find((f) => f.id === status)
  if (status !== null) activeTags.push({ label: activeStatus?.label ?? '', color: activeStatus?.color ?? '' })
  if (isAnime) activeTags.push({ label: 'Anime', color: colors.accentAnilist })
  const activeType = typeFilters.find((f) => f.id === type)
  if (type !== null) activeTags.push({ label: activeType?.label ?? '', color: activeType?.color ?? '' })
  if (showSeriesStatus && seriesStatus !== null) {
    const activeSeries = seriesStatusFilters.find((f) => f.id === seriesStatus)
    activeTags.push({ label: activeSeries?.label ?? '', color: activeSeries?.color ?? '' })
  }
  if (decade) {
    const decadeLabel = decadeOptions.find((o) => o.value === decade)?.label ?? decade
    activeTags.push({ label: decadeLabel, color: colors.accentTeal })
  } else if (releaseFrom || releaseTo) {
    const tag = releaseFrom && releaseTo
      ? `${releaseFrom.slice(0, 7)} → ${releaseTo.slice(0, 7)}`
      : releaseFrom ? `≥ ${releaseFrom}` : `≤ ${releaseTo}`
    activeTags.push({ label: tag, color: colors.accentTeal })
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
              <span
                key={tag.label}
                className={s.activeTag}
                style={{ background: accentWash(tag.color), color: tag.color }}
              >
                {tag.label}
              </span>
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
                    style={active ? { background: accentWash(colors.accentTeal), color: colors.accentTeal } : undefined}
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
          <Chip
            filter={{ id: true, label: 'Anime', color: colors.accentAnilist }}
            active={isAnime}
            onClick={() => onIsAnimeChange(!isAnime)}
          />
          {typeFilters.map((f) => (
            <Chip key={f.label} filter={f} active={type === f.id} onClick={() => onTypeChange(f.id)} />
          ))}
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
