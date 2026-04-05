import { useState, useEffect } from 'preact/hooks'
import clsx from 'clsx'
import type { TitleStatus, TitleType, SeriesStatus } from '../types'
import { colors, accentWash } from '../theme'
import s from './FilterDrawer.module.css'

const STORAGE_KEY_HOME = 'filter-drawer-open-home'

type StatusFilter = TitleStatus | 'up_to_date' | null
type TypeFilter = TitleType | null
type SeriesStatusFilter = SeriesStatus | null

interface FilterDrawerProps {
  status: StatusFilter
  type: TypeFilter
  seriesStatus: SeriesStatusFilter
  onStatusChange: (status: StatusFilter) => void
  onTypeChange: (type: TypeFilter) => void
  onSeriesStatusChange: (seriesStatus: SeriesStatusFilter) => void
  defaultOpen?: boolean
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
  { id: 'anime', label: 'Anime', color: colors.accentAnilist },
  { id: 'movie', label: 'Movie', color: colors.accentAmber },
  { id: 'series', label: 'Series', color: colors.accentLavender },
]

const seriesStatusFilters: { id: SeriesStatusFilter; label: string; color: string }[] = [
  { id: null, label: 'All', color: colors.accentTeal },
  { id: 'returning', label: 'Returning', color: colors.accentGreen },
  { id: 'ended', label: 'Ended', color: colors.textSecondary },
  { id: 'cancelled', label: 'Cancelled', color: colors.accentCoral },
  { id: 'in_production', label: 'In prod.', color: colors.accentTeal },
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
  status, type, seriesStatus,
  onStatusChange, onTypeChange, onSeriesStatusChange,
  defaultOpen = true,
}: FilterDrawerProps) {
  const [open, setOpen] = useState(() => {
    if (!defaultOpen) return false
    const stored = localStorage.getItem(STORAGE_KEY_HOME)
    return stored !== null ? stored === 'true' : true
  })

  // Reset to closed when switching to a page with defaultOpen=false
  useEffect(() => {
    if (!defaultOpen) setOpen(false)
  }, [defaultOpen])

  useEffect(() => {
    if (defaultOpen) localStorage.setItem(STORAGE_KEY_HOME, String(open))
  }, [open, defaultOpen])

  const showSeriesStatus = type === 'series' || type === 'anime'

  // Build active tags for collapsed state
  const activeTags: { label: string; color: string }[] = []
  const activeStatus = statusFilters.find((f) => f.id === status)
  if (status !== null) activeTags.push({ label: activeStatus?.label ?? '', color: activeStatus?.color ?? '' })
  const activeType = typeFilters.find((f) => f.id === type)
  if (type !== null) activeTags.push({ label: activeType?.label ?? '', color: activeType?.color ?? '' })
  if (showSeriesStatus && seriesStatus !== null) {
    const activeSeries = seriesStatusFilters.find((f) => f.id === seriesStatus)
    activeTags.push({ label: activeSeries?.label ?? '', color: activeSeries?.color ?? '' })
  }

  return (
    <>
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
        <div className={clsx(s.filterLabel, s.filterLabelFirst)}>Status</div>
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

        <div className={s.bottomPad} />
      </div>
    </>
  )
}
