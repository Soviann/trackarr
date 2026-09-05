import clsx from 'clsx'
import type { FilterState, FilterActions } from './types'
import { decadeOptions } from './types'
import s from '../FilterDrawer.module.css'

interface FilterDatesTabProps {
  filter: FilterState
  actions: FilterActions
}

export function FilterDatesTab({ filter, actions }: FilterDatesTabProps) {
  const {
    myRatingMin,
    tmdbRatingMin,
    decade,
    releaseFrom,
    releaseTo,
    includeNoRelease,
  } = filter

  return (
    <div className={s.tabPane}>
      <div className={clsx(s.filterLabel, s.filterLabelFirst)}>Rating</div>
      <div className={s.filterRow}>
        <select
          className={s.select}
          value={myRatingMin}
          onChange={(e) => actions.onMyRatingMinChange((e.target as HTMLSelectElement).value)}
        >
          <option value="">My rating: any</option>
          {[1,2,3,4,5,6,7,8,9,10].map(n => (
            <option key={n} value={String(n)}>My rating ≥ {n}</option>
          ))}
        </select>
        <select
          className={s.select}
          value={tmdbRatingMin}
          onChange={(e) => actions.onTmdbRatingMinChange((e.target as HTMLSelectElement).value)}
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
            actions.onDecadeChange(val || null)
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
            actions.onReleaseFromChange((e.target as HTMLInputElement).value)
            if ((e.target as HTMLInputElement).value) actions.onDecadeChange(null)
          }}
        />
        <input
          type="date"
          className={s.dateInput}
          value={releaseTo}
          placeholder="To"
          onChange={(e) => {
            actions.onReleaseToChange((e.target as HTMLInputElement).value)
            if ((e.target as HTMLInputElement).value) actions.onDecadeChange(null)
          }}
        />
      </div>
      {(decade || releaseFrom || releaseTo) && (
        <div className={s.filterRow}>
          <label className={s.toggleLabel}>
            <input
              type="checkbox"
              checked={includeNoRelease}
              onChange={(e) => actions.onIncludeNoReleaseChange((e.target as HTMLInputElement).checked)}
            />
            <span>Include without release date</span>
          </label>
        </div>
      )}
    </div>
  )
}
