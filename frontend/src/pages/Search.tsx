import { useRef, useEffect, useState } from 'preact/hooks'
import { route } from 'preact-router'
import clsx from 'clsx'
import type { Title } from '../types'
import { colors } from '../theme'
import { useTitleStore, useSearchStore } from '../store'
import { getName, getTypeLabel } from '../utils'
import { apiFetch } from '../api'
import { StatusBadge } from '../components/StatusBadge'
import { ErrorBanner } from '../components/ErrorBanner'
import { BottomSheet } from '../components/BottomSheet'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import { PullToRefresh } from '../components/PullToRefresh'
import s from './Search.module.css'

interface TMDBResult {
  id: number
  title: string
  year: number
  poster_url: string | null
}

function getMetadata(t: Title) {
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

export function Search({ path: _ }: { path?: string }) {
  const filter = useTitleStore(s => s.filter)
  const query = useSearchStore(s => s.query)
  const setQuery = useSearchStore(s => s.setQuery)
  const results = useSearchStore(s => s.results)
  const total = useSearchStore(s => s.total)
  const hasMore = useSearchStore(s => s.hasMore)
  const loading = useSearchStore(s => s.loading)
  const loadingMore = useSearchStore(s => s.loadingMore)
  const error = useSearchStore(s => s.error)
  const search = useSearchStore(s => s.search)
  const loadMore = useSearchStore(s => s.loadMore)
  const clear = useSearchStore(s => s.clear)

  const params = new URLSearchParams(typeof window !== 'undefined' ? window.location.search : '')
  const mergeSourceId = params.get('mergeSourceId')
  const mergeSourceName = params.get('mergeSourceName')

  const [mergeTarget, setMergeTarget] = useState<Title | null>(null)
  const [targetSeason, setTargetSeason] = useState(1)
  const [merging, setMerging] = useState(false)
  const [mergeError, setMergeError] = useState<string | null>(null)
  const [searchOnTMDB, setSearchOnTMDB] = useState(false)
  const [tmdbResults, setTmdbResults] = useState<TMDBResult[]>([])

  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    search(filter)
  }, [
    search,
    query,
    filter.status,
    filter.type,
    filter.is_anime,
    filter.series_status,
    filter.decade,
    filter.release_from,
    filter.release_to,
    filter.include_no_release,
  ])

  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  // Load TMDB results when toggle is on. Debounced so each keystroke
  // doesn't burn a TMDB API call.
  useEffect(() => {
    if (!searchOnTMDB || !query.trim()) {
      setTmdbResults([])
      return
    }
    const trimmed = query.trim()
    let cancelled = false
    const timer = setTimeout(() => {
      apiFetch<TMDBResult[]>(`/tmdb/search?query=${encodeURIComponent(trimmed)}&type=movie`)
        .then(r => { if (!cancelled) setTmdbResults(r) })
        .catch(() => { if (!cancelled) setTmdbResults([]) })
    }, 350)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [searchOnTMDB, query])

  const retry = () => search(filter)

  const handleMerge = async () => {
    if (!mergeSourceId || !mergeTarget || merging) return
    setMerging(true)
    setMergeError(null)
    try {
      await apiFetch(`/titles/${mergeSourceId}/merge`, {
        method: 'POST',
        body: JSON.stringify({
          target_id: mergeTarget.id,
          season_offset: mergeTarget.type === 'series' ? targetSeason - 1 : 0,
        }),
      })
      route(`/title/${mergeTarget.id}`)
    } catch (e) {
      setMergeError('Merge failed. Please try again.')
      setMerging(false)
    }
  }

  const hasMatchedAlt = (t: Title) =>
    t.matched_name && t.matched_name !== getName(t)

  return (
    <PullToRefresh onRefresh={retry}>
    <div className={s.page}>
      {/* Results area */}
      <div className={s.results}>
        {!query.trim() && (
          <div className={s.emptyState}>
            <div className={s.emptyInner}>
              <div className={s.emptyIcon}>
                <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke={colors.inkDim} stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                  <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
                </svg>
              </div>
              <div className={s.emptyText}>
                {mergeSourceId ? 'Search for a title to merge into' : 'Search across your entire library'}
              </div>
              <div className={s.emptySubtext}>
                {mergeSourceId ? `Merging "${mergeSourceName}"` : 'All statuses, all types'}
              </div>
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
              {results.filter(t => t.id !== Number(mergeSourceId)).map((t) => (
                <div
                  key={t.id}
                  onClick={() => {
                    if (mergeSourceId) {
                      setMergeTarget(t)
                      setTargetSeason(1)
                    } else {
                      route(`/title/${t.id}`)
                    }
                  }}
                  className={s.card}
                >
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
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.inkDim} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    {mergeSourceId ? (
                      <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" stroke={colors.accent} />
                    ) : (
                      <polyline points="9 18 15 12 9 6" />
                    )}
                  </svg>
                </div>
              ))}
            </div>

            {hasMore && (
              <div className={s.loadMoreWrap}>
                <button onClick={() => loadMore(filter)} disabled={loadingMore} className={s.loadMoreBtn}>
                  {loadingMore ? 'Loading...' : 'Load more'}
                </button>
              </div>
            )}

            {/* TMDB results — only when toggle is on */}
            {searchOnTMDB && tmdbResults.length > 0 && (
              <>
                <div className={s.sectionDivider}>TMDB Results</div>
                {tmdbResults
                  .filter(r => !results.some(l => l.tmdb_id === r.id))
                  .map(r => (
                    <div key={r.id} className={clsx(s.card, s.cardTMDB)}>
                      <div
                        className={s.cardCover}
                        style={{ background: r.poster_url ? `url(${r.poster_url}) center/cover` : '#333' }}
                      />
                      <div className={s.cardBody}>
                        <div className={s.cardHeader}>
                          <span className={s.cardTitle}>{r.title}</span>
                        </div>
                        <div className={s.cardMeta}>{r.year || '—'}</div>
                      </div>
                    </div>
                  ))}
              </>
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

      <BottomSheet open={!!mergeTarget} onClose={() => setMergeTarget(null)} ariaLabel="Merge titles">
        <div className={s.mergeDrawer}>
          <div className={s.mergeTitle}>Merge titles?</div>
          <div className={s.mergeDesc}>
            This will merge "{mergeSourceName}" into "{mergeTarget ? getName(mergeTarget) : ''}".
            Seasons, watch events and names will be moved. This action cannot be undone.
          </div>

          {mergeTarget?.type === 'series' && (
            <div className={s.seasonInputGroup}>
              <label htmlFor="target-season" className={s.seasonLabel}>Integrate as season number:</label>
              <input
                id="target-season"
                type="number"
                min="1"
                value={targetSeason}
                onInput={(e) => setTargetSeason(Number((e.target as HTMLInputElement).value))}
                className={s.seasonInput}
              />
            </div>
          )}

          {mergeError && <div className={s.mergeError}>{mergeError}</div>}

          <div className={s.mergeActions}>
            <button className={s.cancelBtn} onClick={() => setMergeTarget(null)}>Cancel</button>
            <button
              className={s.confirmBtn}
              onClick={handleMerge}
              disabled={merging}
            >
              {merging ? 'Merging...' : 'Merge now'}
            </button>
          </div>
        </div>
      </BottomSheet>

      {/* Search input */}
      <div className={s.searchBar}>
        <div className={clsx(s.searchInner, query ? s.searchInnerFocused : s.searchInnerIdle)}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.accent} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
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
            <button type="button" onClick={clear} aria-label="Clear search" className={s.clearBtn}>
              <svg
                width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.inkDim}
                stroke-width="2" stroke-linecap="round" stroke-linejoin="round"
              >
                <line x1="18" y1="6" x2="6" y2="18" /><line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          )}
          {!mergeSourceId && (
            <button
              className={clsx(s.tmdbToggle, searchOnTMDB && s.tmdbToggleOn)}
              onClick={() => setSearchOnTMDB(v => !v)}
              aria-pressed={searchOnTMDB}
              title="Also search TMDB"
            >
              TMDB
            </button>
          )}
        </div>
      </div>
    </div>
    </PullToRefresh>
  )
}
