import type { ComponentChildren } from 'preact'
import { useEffect, useRef } from 'preact/hooks'
import { route } from 'preact-router'
import clsx from 'clsx'
import { useTranslation } from '../i18n'
import { useSearchStore } from '../store'
import { detectUrlType, isUrl } from '../utils/url'
import { routeTo } from '../routes'
import s from './SearchBar.module.css'

interface SearchBarProps {
  showTMDBToggle?: boolean
  isFiltersOpen?: boolean
  onToggleFilters?: () => void
  activeFilterCount?: number
  variant?: 'docked' | 'top'
  className?: string
  children?: ComponentChildren
}

export function SearchBar({
  showTMDBToggle = true,
  isFiltersOpen = false,
  onToggleFilters,
  activeFilterCount = 0,
  variant = 'docked',
  className,
  children,
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

  const handleInputChange = (val: string) => {
    const trimmed = val.trim()
    if (detectUrlType(trimmed) || (isUrl(trimmed) && (trimmed.includes('themoviedb.org') || trimmed.includes('imdb.com') || trimmed.includes('anilist.co') || trimmed.includes('thetvdb.com')))) {
      route(`${routeTo.adminValidate()}?q=${encodeURIComponent(trimmed)}`)
      return
    }
    setQuery(val)
  }

  return (
    <search className={clsx(s.searchBar, variant === 'top' && s.searchBarTop, className)} role="search">
      <div className={clsx(s.searchInner, query ? s.searchInnerFocused : s.searchInnerIdle)}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--ink-dim)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <input
          ref={inputRef}
          type="text"
          name="search"
          id="search"
          aria-label={t('search.placeholder')}
          autocomplete="off"
          value={query}
          onInput={(e) => handleInputChange((e.target as HTMLInputElement).value)}
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
        {children}
      </div>
    </search>
  )
}
