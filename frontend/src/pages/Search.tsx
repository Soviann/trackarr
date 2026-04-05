import { useState, useRef, useEffect, useCallback } from 'preact/hooks'
import { route } from 'preact-router'
import clsx from 'clsx'
import type { Title, TitleStatus, PaginatedResponse } from '../types'
import { colors, accentWash } from '../theme'
import { apiFetch } from '../api'
import { getName, getTypeLabel } from '../utils'
import { StatusBadge } from '../components/StatusBadge'
import { ErrorBanner } from '../components/ErrorBanner'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import s from './Search.module.css'

const PAGE_SIZE = 50

const statusFilters: { id: TitleStatus | null; label: string; color: string }[] = [
  { id: null, label: 'All', color: colors.accentTeal },
  { id: 'watching', label: 'Watching', color: colors.accentAmber },
  { id: 'completed', label: 'Completed', color: colors.accentGreen },
  { id: 'dropped', label: 'Dropped', color: colors.accentCoral },
  { id: 'plan_to_watch', label: 'Plan', color: colors.textSecondary },
]

export function Search({ path: _ }: { path?: string }) {
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<TitleStatus | null>(null)
  const [results, setResults] = useState<Title[]>([])
  const [total, setTotal] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const buildUrl = useCallback((offset: number) => {
    const trimmed = query.trim()
    if (!trimmed) return null
    const params = new URLSearchParams()
    params.set('search', trimmed)
    if (statusFilter) params.set('status', statusFilter)
    params.set('limit', String(PAGE_SIZE))
    params.set('offset', String(offset))
    return `/titles?${params.toString()}`
  }, [query, statusFilter])

  // Fetch first page
  useEffect(() => {
    const url = buildUrl(0)
    if (!url) {
      setResults([])
      setTotal(0)
      setHasMore(false)
      return
    }
    setLoading(true)
    setError(null)
    apiFetch<PaginatedResponse>(url)
      .then((r) => {
        setResults(r.titles)
        setTotal(r.total)
        setHasMore(r.has_more)
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }, [buildUrl])

  const handleLoadMore = async () => {
    const url = buildUrl(results.length)
    if (!url || loadingMore) return
    setLoadingMore(true)
    try {
      const r = await apiFetch<PaginatedResponse>(url)
      setResults((prev) => [...prev, ...r.titles])
      setHasMore(r.has_more)
      setTotal(r.total)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Load more failed')
    } finally {
      setLoadingMore(false)
    }
  }

  const retry = () => {
    const url = buildUrl(0)
    if (!url) return
    setLoading(true)
    setError(null)
    apiFetch<PaginatedResponse>(url)
      .then((r) => {
        setResults(r.titles)
        setTotal(r.total)
        setHasMore(r.has_more)
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

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
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#2A2A2A" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
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

        {query.trim() && results.length > 0 && (
          <>
            <div className={s.resultCount}>
              <span className={s.resultCountText}>
                {results.length} / {total} result{total !== 1 ? 's' : ''} for "{query.trim()}"
              </span>
            </div>

            {/* Status filter chips */}
            <div className={s.filterChips}>
              {statusFilters.map((sf) => {
                const isActive = statusFilter === sf.id
                return (
                  <button
                    key={sf.id ?? 'all'}
                    onClick={() => setStatusFilter(sf.id)}
                    className={clsx(s.chip, isActive && s.chipActive)}
                    style={isActive ? { background: accentWash(sf.color), color: sf.color } : undefined}
                  >
                    {sf.label}
                  </button>
                )
              })}
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
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#333" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="9 18 15 12 9 6" />
                  </svg>
                </div>
              ))}
            </div>

            {hasMore && (
              <div className={s.loadMoreWrap}>
                <button onClick={handleLoadMore} disabled={loadingMore} className={s.loadMoreBtn}>
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

        {query.trim() && loading && (
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
              onClick={() => { setQuery(''); setStatusFilter(null) }}
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
