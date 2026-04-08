import { useRef, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import clsx from 'clsx'
import type { Title } from '../types'
import { colors } from '../theme'
import { useTitleStore, useSearchStore } from '../store'
import { getName, getTypeLabel } from '../utils'
import { StatusBadge } from '../components/StatusBadge'
import { ErrorBanner } from '../components/ErrorBanner'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import s from './Search.module.css'

export function Search({ path: _ }: { path?: string }) {
  const { filter } = useTitleStore()
  const {
    query, setQuery, results, total, hasMore,
    loading, loadingMore, error,
    search, loadMore, clear
  } = useSearchStore()
  
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    search(filter)
  }, [query, filter.status, filter.type, filter.series_status])

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const retry = () => search(filter)

  const getMetadata = (t: Title) => {
    const parts = [getTypeLabel(t.type), String(t.year)]
    const seasons = t.seasons ?? []
    if (t.type !== 'movie' && seasons.length > 0) {
      const ss = seasons[seasons.length - 1]
      const w = ss.watched_count ?? (ss.episodes ?? []).filter((e) => e.watched).length
      const totalEp = ss.total_episodes ?? ss.episode_count ?? (ss.episodes ?? []).length
      parts.push(`S${ss.season_number} ${w}/${totalEp}`)
    }
    if (t.my_rating) parts.push(`\u2605 ${t.my_rating}`)
    return parts.join(' \u00b7 ')
  }

  const hasMatchedAlt = (t: Title) =>
    t.matched_name && t.matched_name !== getName(t)

  return (
    <div className={s.page}>
      {/* Results area */}
      <div className={s.results}>
        {!query.trim() && (
          <div className={s.emptyState}>
            <div className={s.emptyInner}>
              <div className={s.emptyIcon}>
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#888" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
              </div>
              <div className={s.emptyText}>
                Search across your entire library
              </div>
              <div className={s.emptySubtext}>All statuses, all types</div>
            </div>
          </div>
        )}

        {query.trim() && error && <ErrorBanner message={error} onRetry={retry} />}

        {results.length > 0 && (
          <>
            <div className={s.resultCount}>
              <span className={s.resultCountText}>
                {results.length} / {total} result{total !== 1 ? 's' : ''} for "{query.trim()}"
              </span>
            </div>

            <div className={s.cardList}>
              {results.map((t) => (
                <div key={t.id} onClick={() => route(`/title/${t.id}`)} className={s.card}>
                  <div
                    className={s.cardCover}
                    style={{ background: coverBackground(t.cover_url, t.type) }}
                  >
                    {!t.cover_url && <CoverPlaceholder type={t.type} iconSize="18px" />}
                  </div>
                  <div className={s.cardBody}>
                    <div className={s.cardHeader}>
                      <span className={s.cardTitle}>{getName(t)}</span>
                      <StatusBadge status={t.status} />
                    </div>
                    {hasMatchedAlt(t) && (
                      <div className={s.matchedRow}>
                        <span className={s.matchedName}>{t.matched_name}</span>
                        {t.matched_language && (
                          <span className={s.matchedLang}>{t.matched_language}</span>
                        )}
                      </div>
                    )}
                    <div className={s.cardMeta}>{getMetadata(t)}</div>
                  </div>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#888" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="9 18 15 12 9 6" />
                  </svg>
                </div>
              ))}
            </div>

            {hasMore && (
              <div className={s.loadMoreWrap}>
                <button onClick={() => loadMore(filter)} disabled={loadingMore} className={s.loadMoreBtn}>
                  {loadingMore ? 'Chargement...' : 'Charger plus'}
                </button>
              </div>
            )}
          </>
        )}

        {query.trim() && !loading && results.length === 0 && !error && (
          <div className={s.statusMessage}>
            No results for "{query.trim()}"
          </div>
        )}

        {query.trim() && loading && results.length === 0 && (
          <div className={s.statusMessage}>
            Searching...
          </div>
        )}
      </div>

      {/* Search input */}
      <div className={s.searchBar}>
        <div className={clsx(s.searchInner, query ? s.searchInnerFocused : s.searchInnerIdle)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.accentTeal} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
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
            placeholder="Search titles..."
            className={s.searchInput}
          />
          {query && (
            <svg
              onClick={clear}
              width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.textMuted}
              stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
              className={s.clearBtn}
            >
              <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          )}
        </div>
      </div>
    </div>
  )
}
