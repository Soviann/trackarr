import clsx from 'clsx'
import type { SortState, SortField } from '../../store'
import type {
  FilterState,
  FilterActions,
  StatusFilter,
  SeriesStatusFilter,
} from './types'
import {
  sortOptions,
  statusFilters,
  typeFilters,
  seriesStatusFilters,
} from './types'
import s from '../FilterDrawer.module.css'

interface FilterBasicsTabProps {
  filter: FilterState
  actions: FilterActions
  sort: SortState
  onSortChange: (sort: SortState) => void
  isSearchActive: boolean
}

export function FilterBasicsTab({
  filter,
  actions,
  sort,
  onSortChange,
  isSearchActive,
}: FilterBasicsTabProps) {
  const showSeriesStatus = filter.type === 'series'
  const isSortActive = !isSearchActive && (sort.field !== 'updated_at' || sort.order !== 'desc')

  return (
    <div className={s.tabPane}>
      {!isSearchActive && (
        <div className={s.filterRow}>
          <div className={s.sortCombo}>
            <select
              aria-label="Sort by"
              className={clsx(s.select, s.sortSelect, isSortActive && s.selectActive)}
              value={sort.field}
              onChange={(e) => {
                const field = (e.target as HTMLSelectElement).value as SortField
                const opt = sortOptions.find((o) => o.field === field)
                onSortChange({ field, order: opt ? opt.defaultOrder : 'desc' })
              }}
            >
              {sortOptions.map((opt) => (
                <option key={opt.field} value={opt.field}>
                  {`Sort: ${opt.label}`}
                </option>
              ))}
            </select>
            <button
              type="button"
              className={clsx(s.orderBtn, sort.order === 'asc' && s.orderBtnAsc)}
              title={sort.order === 'asc' ? 'Ascending' : 'Descending'}
              aria-label={sort.order === 'asc' ? 'Ascending' : 'Descending'}
              onClick={() => onSortChange({ field: sort.field, order: sort.order === 'asc' ? 'desc' : 'asc' })}
            >
              {sort.order === 'asc' ? '↑' : '↓'}
            </button>
          </div>
        </div>
      )}

      <div className={clsx(s.filterRow, s.twoColRow)}>
        {/* Status Select */}
        <select
          aria-label="Filter status"
          className={clsx(s.select, s.statusSelect, filter.status !== null && s.selectActive)}
          value={filter.status ?? ''}
          onChange={(e) => {
            const val = (e.target as HTMLSelectElement).value
            actions.onStatusChange(val ? (val as StatusFilter) : null)
          }}
        >
          <option value="">Status: All</option>
          {statusFilters
            .filter((f) => f.id !== null)
            .map((f) => (
              <option key={f.id} value={f.id!}>
                {`Status: ${f.label}`}
              </option>
            ))}
        </select>

        {/* Type Segmented Control + Anime */}
        <div className={s.typeGroup}>
          <div className={s.segmentedControl} role="group" aria-label="Filter type">
            {typeFilters.map((f) => (
              <button
                key={f.label}
                type="button"
                className={clsx(s.segmentBtn, filter.type === f.id && s.segmentBtnActive)}
                onClick={() => actions.onTypeChange(f.id)}
              >
                {f.label}
              </button>
            ))}
          </div>
          <button
            type="button"
            className={clsx(s.animePill, filter.isAnime && s.animePillActive)}
            title="Filter Anime"
            aria-pressed={filter.isAnime}
            onClick={() => actions.onIsAnimeChange(!filter.isAnime)}
          >
            <span>⛩</span>
          </button>
        </div>
      </div>

      {showSeriesStatus && (
        <div className={s.filterRow}>
          <select
            aria-label="Filter series status"
            className={clsx(s.select, filter.seriesStatus !== null && s.selectActive)}
            value={filter.seriesStatus ?? ''}
            onChange={(e) => {
              const val = (e.target as HTMLSelectElement).value
              actions.onSeriesStatusChange(val ? (val as SeriesStatusFilter) : null)
            }}
          >
            <option value="">Series status: All</option>
            {seriesStatusFilters
              .filter((f) => f.id !== null)
              .map((f) => (
                <option key={f.id} value={f.id!}>
                  {`Series: ${f.label}`}
                </option>
              ))}
          </select>
        </div>
      )}
    </div>
  )
}
