import clsx from 'clsx'
import type { SortState } from '../../store'
import type {
  FilterState,
  FilterActions,
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

export function FilterBasicsTab({
  filter,
  actions,
  sort,
  onSortChange,
  isSearchActive,
}: FilterBasicsTabProps) {
  const showSeriesStatus = filter.type === 'series'

  const handleSortClick = (option: typeof sortOptions[number]) => {
    if (sort.field === option.field) {
      onSortChange({ field: sort.field, order: sort.order === 'asc' ? 'desc' : 'asc' })
    } else {
      onSortChange({ field: option.field, order: option.defaultOrder })
    }
  }

  return (
    <div className={s.tabPane}>
      {!isSearchActive && (
        <>
          <div className={clsx(s.filterLabel, s.filterLabelFirst)}>Sort</div>
          <div className={s.filterRow}>
            {sortOptions.map((opt) => {
              const active = sort.field === opt.field
              return (
                <button
                  key={opt.field}
                  type="button"
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
          <Chip
            key={f.label}
            filter={f}
            active={filter.status === f.id}
            onClick={() => actions.onStatusChange(f.id)}
          />
        ))}
      </div>

      <div className={s.filterLabel}>Type</div>
      <div className={s.filterRow}>
        {typeFilters.map((f) => (
          <Chip
            key={f.label}
            filter={f}
            active={filter.type === f.id}
            onClick={() => actions.onTypeChange(f.id)}
          />
        ))}
        <Chip
          filter={{ id: true, label: 'Anime' }}
          active={filter.isAnime}
          onClick={() => actions.onIsAnimeChange(!filter.isAnime)}
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
                active={filter.seriesStatus === f.id}
                onClick={() => actions.onSeriesStatusChange(f.id)}
              />
            ))}
          </div>
        </>
      )}
    </div>
  )
}
