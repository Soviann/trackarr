import { useState, useRef, useEffect, useCallback } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title, TitleStatus, PaginatedResponse } from '../types'
import { colors, accentWash } from '../theme'
import { apiFetch } from '../api'
import { getName, getTypeLabel } from '../utils'
import { StatusBadge } from '../components/StatusBadge'
import { ErrorBanner } from '../components/ErrorBanner'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'

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
      const s = seasons[seasons.length - 1]
      const w = s.watched_count ?? (s.episodes ?? []).filter((e) => e.watched).length
      const totalEp = s.total_episodes ?? s.episode_count ?? (s.episodes ?? []).length
      parts.push(`S${s.season_number} ${w}/${totalEp}`)
    }
    if (t.my_rating) parts.push(`\u2605 ${t.my_rating}`)
    return parts.join(' \u00b7 ')
  }

  const hasMatchedAlt = (t: Title) =>
    t.matched_name && t.matched_name !== getName(t)

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: 'calc(100vh - 108px)' }}>
      {/* Results area */}
      <div style={{ flex: 1 }}>
        {!query.trim() && (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            minHeight: 'calc(100vh - 200px)',
          }}>
            <div style={{ textAlign: 'center', padding: '0 32px' }}>
              <div style={{
                width: '56px', height: '56px', borderRadius: '50%',
                background: colors.bgCard,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
                margin: '0 auto 16px',
              }}>
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#2A2A2A" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
              </div>
              <div style={{ fontSize: '13px', color: colors.textDimmed, lineHeight: '1.6' }}>
                Search across your entire library
              </div>
              <div style={{ fontSize: '11px', color: '#333', marginTop: '4px' }}>All statuses, all types</div>
            </div>
          </div>
        )}

        {query.trim() && error && <ErrorBanner message={error} onRetry={retry} />}

        {query.trim() && results.length > 0 && (
          <>
            <div style={{ padding: '16px 16px 8px' }}>
              <span style={{ fontSize: '10px', color: colors.textMuted }}>
                {results.length} / {total} result{total !== 1 ? 's' : ''} for "{query.trim()}"
              </span>
            </div>

            {/* Status filter chips */}
            <div style={{
              display: 'flex',
              gap: '6px',
              padding: '0 16px 12px',
              overflowX: 'auto',
              WebkitOverflowScrolling: 'touch',
            }}>
              {statusFilters.map((sf) => {
                const isActive = statusFilter === sf.id
                return (
                  <button
                    key={sf.id ?? 'all'}
                    onClick={() => setStatusFilter(sf.id)}
                    style={{
                      fontSize: '10px',
                      fontWeight: isActive ? 600 : 400,
                      padding: '4px 10px',
                      borderRadius: '12px',
                      border: 'none',
                      whiteSpace: 'nowrap',
                      cursor: 'pointer',
                      background: isActive ? accentWash(sf.color) : colors.bgSurface,
                      color: isActive ? sf.color : colors.textMuted,
                      WebkitTapHighlightColor: 'transparent',
                    }}
                  >
                    {sf.label}
                  </button>
                )
              })}
            </div>

            <div style={{ padding: '0 16px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {results.map((t) => (
                <div
                  key={t.id}
                  onClick={() => route(`/title/${t.id}`)}
                  style={{
                    display: 'flex',
                    gap: '12px',
                    alignItems: 'center',
                    background: colors.bgCard,
                    borderRadius: '10px',
                    padding: '10px 12px',
                    border: `1px solid ${colors.borderCard}`,
                    cursor: 'pointer',
                  }}
                >
                  <div style={{
                    width: '42px', height: '60px', borderRadius: '6px', flexShrink: 0,
                    background: coverBackground(t.cover_url, t.type),
                    position: 'relative', overflow: 'hidden',
                  }}>
                    {!t.cover_url && <CoverPlaceholder type={t.type} iconSize="18px" />}
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <span style={{
                        fontSize: '13px', fontWeight: 600, color: colors.textPrimary,
                        overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap',
                      }}>
                        {getName(t)}
                      </span>
                      <StatusBadge status={t.status} />
                    </div>
                    {hasMatchedAlt(t) && (
                      <div style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: '4px',
                        marginTop: '2px',
                      }}>
                        <span style={{
                          fontSize: '11px',
                          color: colors.textSecondary,
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}>
                          {t.matched_name}
                        </span>
                        {t.matched_language && (
                          <span style={{
                            fontSize: '8px',
                            color: colors.textMuted,
                            background: colors.bgSurface,
                            borderRadius: '3px',
                            padding: '1px 4px',
                            fontWeight: 600,
                            textTransform: 'uppercase',
                            flexShrink: 0,
                          }}>
                            {t.matched_language}
                          </span>
                        )}
                      </div>
                    )}
                    <div style={{ fontSize: '10px', color: colors.textSecondary, marginTop: '3px' }}>
                      {getMetadata(t)}
                    </div>
                  </div>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#333" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="9 18 15 12 9 6" />
                  </svg>
                </div>
              ))}
            </div>

            {hasMore && (
              <div style={{ padding: '12px 16px 24px', textAlign: 'center' }}>
                <button
                  onClick={handleLoadMore}
                  disabled={loadingMore}
                  style={{
                    background: colors.bgCard,
                    border: `1px solid ${colors.borderCard}`,
                    borderRadius: '10px',
                    padding: '10px 24px',
                    color: colors.accentTeal,
                    fontSize: '12px',
                    fontWeight: 600,
                    cursor: loadingMore ? 'default' : 'pointer',
                    opacity: loadingMore ? 0.6 : 1,
                    fontFamily: 'inherit',
                  }}
                >
                  {loadingMore ? 'Chargement...' : 'Charger plus'}
                </button>
              </div>
            )}
          </>
        )}

        {query.trim() && !loading && results.length === 0 && !error && (
          <div style={{ padding: '40px 16px', textAlign: 'center', color: colors.textSecondary }}>
            No results for "{query.trim()}"
          </div>
        )}

        {query.trim() && loading && (
          <div style={{ padding: '40px 16px', textAlign: 'center', color: colors.textSecondary }}>
            Searching...
          </div>
        )}
      </div>

      {/* Search input */}
      <div style={{
        padding: '8px 16px',
        borderTop: `1px solid ${colors.borderSubtle}`,
        position: 'fixed',
        bottom: '72px',
        left: 0,
        right: 0,
        background: colors.bgPrimary,
        zIndex: 99,
      }}>
        <div style={{
          background: colors.bgSurface,
          borderRadius: '12px',
          padding: '10px 14px',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          border: query ? 'none' : `1.5px solid rgba(56,189,176,0.3)`,
        }}>
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
            style={{
              flex: 1,
              fontSize: '14px',
              color: colors.textPrimary,
              background: 'transparent',
              border: 'none',
              outline: 'none',
              fontFamily: 'inherit',
            }}
          />
          {query && (
            <svg
              onClick={() => { setQuery(''); setStatusFilter(null) }}
              width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.textMuted}
              stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
              style={{ cursor: 'pointer', flexShrink: 0 }}
            >
              <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          )}
        </div>
      </div>
    </div>
  )
}
