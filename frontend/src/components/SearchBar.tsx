import { useEffect, useRef } from 'preact/hooks'
import clsx from 'clsx'
import { colors } from '../theme'
import { useTranslation } from '../i18n'
import { useSearchStore } from '../store'
import s from './SearchBar.module.css'

interface SearchBarProps {
  showTMDBToggle?: boolean
  isFiltersOpen?: boolean
  onToggleFilters?: () => void
  activeFilterCount?: number
}

export function SearchBar({
  showTMDBToggle = true,
  isFiltersOpen = false,
  onToggleFilters,
  activeFilterCount = 0,
}: SearchBarProps) {
  const { t } = useTranslation()
  const query = useSearchStore(s => s.query)
  const setQuery = useSearchStore(s => s.setQuery)
  const clear = useSearchStore(s => s.clear)
  const searchOnTMDB = useSearchStore(s => s.searchOnTMDB)
  const setSearchOnTMDB = useSearchStore(s => s.setSearchOnTMDB)

  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  return (
    <div className={s.searchBar}>
      <div className={clsx(s.searchInner, query ? s.searchInnerFocused : s.searchInnerIdle)}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.inkDim} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input
          ref={inputRef}
          type="text"
          name="search"
          id="search"
          autocomplete="off"
          value={query}
          onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
          placeholder={t('search.placeholder')}
          className={s.searchInput}
        />
        {query && (
          <button
            type="button"
            onClick={clear}
            aria-label={t('search.clearText')}
            title={t('search.clearText')}
            className={s.clearBtn}
          >
            <svg
              width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor"
              stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"
            >
              <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        )}
        {showTMDBToggle && (
          <button
            type="button"
            className={clsx(s.tmdbToggle, searchOnTMDB && s.tmdbToggleOn)}
            onClick={() => setSearchOnTMDB(!searchOnTMDB)}
            aria-pressed={searchOnTMDB}
            title={t('search.tmdbToggleTitle')}
          >
            {t('search.tmdbToggle')}
          </button>
        )}
        {onToggleFilters && (
          <button
            type="button"
            className={clsx(
              s.filterTriggerBtn,
              activeFilterCount > 0 && s.filterTriggerBtnActive,
              isFiltersOpen && s.filterTriggerBtnOpen
            )}
            onClick={onToggleFilters}
            aria-expanded={isFiltersOpen}
            title={t('search.filters')}
          >
            <span>{t('search.filters')}</span>
            {activeFilterCount > 0 && (
              <span className={s.filterCountBadge}>{activeFilterCount}</span>
            )}
          </button>
        )}
      </div>
    </div>
  )
}

