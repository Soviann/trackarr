import clsx from 'clsx'
import { useTranslation } from '../../i18n'
import type { FilterState, FilterActions } from './types'
import { decadeOptions } from './types'
import s from '../FilterDrawer.module.css'

interface FilterDatesTabProps {
  filter: FilterState
  actions: FilterActions
}

export function FilterDatesTab({ filter, actions }: FilterDatesTabProps) {
  const { t } = useTranslation()
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
      <div className={clsx(s.filterLabel, s.filterLabelFirst)}>{t('library.sortRating')}</div>
      <div className={s.filterRow}>
        <select
          className={clsx(s.select, Boolean(myRatingMin) && s.selectActive)}
          value={myRatingMin}
          onChange={(e) => actions.onMyRatingMinChange((e.target as HTMLSelectElement).value)}
        >
          <option value="">{t('search.myRatingAny')}</option>
          {[1,2,3,4,5,6,7,8,9,10].map(n => (
            <option key={n} value={String(n)}>{t('search.myRatingMin', { n })}</option>
          ))}
        </select>
        <select
          className={clsx(s.select, Boolean(tmdbRatingMin) && s.selectActive)}
          value={tmdbRatingMin}
          onChange={(e) => actions.onTmdbRatingMinChange((e.target as HTMLSelectElement).value)}
        >
          <option value="">{t('search.tmdbRatingAny')}</option>
          {[5,6,7,8,9].map(n => (
            <option key={n} value={String(n)}>{t('search.tmdbRatingMin', { n })}</option>
          ))}
        </select>
      </div>

      <div className={s.filterLabel}>{t('search.releaseDate')}</div>
      <div className={s.filterRow}>
        <select
          className={clsx(s.select, Boolean(decade) && s.selectActive)}
          value={decade ?? ''}
          onChange={(e) => {
            const val = (e.target as HTMLSelectElement).value
            actions.onDecadeChange(val || null)
          }}
        >
          {decadeOptions.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.value === '' ? t('search.decadeAll') : opt.label}
            </option>
          ))}
        </select>
        <input
          type="date"
          className={s.dateInput}
          value={releaseFrom}
          placeholder={t('search.dateFrom')}
          aria-label={t('search.dateFrom')}
          onChange={(e) => {
            actions.onReleaseFromChange((e.target as HTMLInputElement).value)
            if ((e.target as HTMLInputElement).value) actions.onDecadeChange(null)
          }}
        />
        <input
          type="date"
          className={s.dateInput}
          value={releaseTo}
          placeholder={t('search.dateTo')}
          aria-label={t('search.dateTo')}
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
            <span>{t('search.includeWithoutRelease')}</span>
          </label>
        </div>
      )}
    </div>
  )
}
