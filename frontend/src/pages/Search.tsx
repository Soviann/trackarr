import { useEffect, useMemo, useState } from 'preact/hooks'
import { route } from 'preact-router'
import clsx from 'clsx'
import type { Title, TitleStatus } from '../types'
import { colors } from '../theme'
import { useTranslation } from '../i18n'
import { useTitleStore, useSearchStore } from '../store'
import { getName, getTypeLabel } from '../utils'
import { apiFetch } from '../api'
import { routeTo } from '../routes'
import { useUndo } from '../context/UndoContext'
import { StatusBadge } from '../components/StatusBadge'
import { TypeBadge } from '../components/TypeBadge'
import { ArrBadge } from '../components/ArrBadge'
import { ErrorBanner } from '../components/ErrorBanner'
import { BottomSheet } from '../components/BottomSheet'
import { CoverImage } from '../components/CoverImage'
import { PullToRefresh } from '../components/PullToRefresh'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import s from './Search.module.css'

interface DiscoveryItem {
  id: string
  title: string
  year: number
  type: 'movie' | 'series'
  isAnime: boolean
  posterUrl: string | null
  source: 'TMDB' | 'AniList'
  tmdbId?: number
  anilistId?: number
  localTitleId?: number
  adding?: boolean
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

export function Search({ path: _, filterOpen = false }: { path?: string; filterOpen?: boolean }) {
  const { t } = useTranslation()
  const { showUndo } = useUndo()
  const filter = useTitleStore(s => s.filter)
  const query = useSearchStore(s => s.query)
  const results = useSearchStore(s => s.results)
  const total = useSearchStore(s => s.total)
  const hasMore = useSearchStore(s => s.hasMore)
  const loading = useSearchStore(s => s.loading)
  useScrollRestoration('search', !loading)
  const loadingMore = useSearchStore(s => s.loadingMore)
  const error = useSearchStore(s => s.error)
  const search = useSearchStore(s => s.search)
  const loadMore = useSearchStore(s => s.loadMore)
  const searchOnTMDB = useSearchStore(s => s.searchOnTMDB)
  const setSearchOnTMDB = useSearchStore(s => s.setSearchOnTMDB)

  const params = new URLSearchParams(typeof window !== 'undefined' ? window.location.search : '')
  const mergeSourceId = params.get('mergeSourceId')
  const mergeSourceName = params.get('mergeSourceName')

  const [mergeTarget, setMergeTarget] = useState<Title | null>(null)
  const [targetSeason, setTargetSeason] = useState(1)
  const [merging, setMerging] = useState(false)
  const [mergeError, setMergeError] = useState<string | null>(null)
  const [discoveryResults, setDiscoveryResults] = useState<DiscoveryItem[]>([])
  const [loadingDiscovery, setLoadingDiscovery] = useState(false)

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
    filter.genres,
    filter.genre_op,
    filter.origin_country,
    filter.my_rating_min,
    filter.tmdb_rating_min,
  ])

  // Load TMDB & AniList discovery results when toggle is on. Debounced.
  useEffect(() => {
    if (!searchOnTMDB || !query.trim()) {
      setDiscoveryResults([])
      setLoadingDiscovery(false)
      return
    }
    const trimmed = query.trim()
    let cancelled = false
    setLoadingDiscovery(true)
    const timer = setTimeout(async () => {
      try {
        const [tmdbRes, anilistRes] = await Promise.allSettled([
          apiFetch<Array<{ id: number; title: string; year: number; poster_url: string | null; type?: string; overview?: string }>>(
            `/tmdb/search?query=${encodeURIComponent(trimmed)}&type=all`
          ),
          apiFetch<Array<{ id: number; title: string; year?: number | null; format?: string; poster_url?: string | null; english_title?: string; romaji_title?: string }>>(
            `/anilist/search?query=${encodeURIComponent(trimmed)}`
          ),
        ])

        if (cancelled) return

        const items: DiscoveryItem[] = []

        if (tmdbRes.status === 'fulfilled' && Array.isArray(tmdbRes.value)) {
          for (const tmdb of tmdbRes.value) {
            const isSeries = tmdb.type === 'tv'
            const item: DiscoveryItem = {
              id: `tmdb-${tmdb.id}`,
              title: tmdb.title,
              year: tmdb.year,
              type: isSeries ? 'series' : 'movie',
              isAnime: false,
              posterUrl: tmdb.poster_url,
              source: 'TMDB',
              tmdbId: tmdb.id,
            }
            items.push(item)
          }
        }

        if (anilistRes.status === 'fulfilled' && Array.isArray(anilistRes.value)) {
          for (const al of anilistRes.value) {
            const itemTitle = al.title || al.english_title || al.romaji_title || ''
            if (!itemTitle) continue
            if (items.some(it => it.title.toLowerCase() === itemTitle.toLowerCase() && it.year === (al.year ?? 0))) continue

            const item: DiscoveryItem = {
              id: `anilist-${al.id}`,
              title: itemTitle,
              year: al.year ?? 0,
              type: al.format === 'MOVIE' ? 'movie' : 'series',
              isAnime: true,
              posterUrl: al.poster_url ?? null,
              source: 'AniList',
              anilistId: al.id,
            }
            items.push(item)
          }
        }

        setDiscoveryResults(items)
      } catch {
        if (!cancelled) setDiscoveryResults([])
      } finally {
        if (!cancelled) setLoadingDiscovery(false)
      }
    }, 350)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [searchOnTMDB, query])

  const discoveryItems = useMemo(() => {
    return discoveryResults.map((item) => {
      if (item.localTitleId) return item
      const localHit = results.find(
        (t) =>
          (item.tmdbId && t.tmdb_id === item.tmdbId) ||
          (item.anilistId && t.anilist_id === item.anilistId) ||
          (getName(t).toLowerCase() === item.title.toLowerCase() && (!item.year || !t.year || t.year === item.year))
      )
      if (localHit) {
        return { ...item, localTitleId: localHit.id }
      }
      return item
    })
  }, [discoveryResults, results])

  const handleAddDiscovery = async (item: DiscoveryItem, status: TitleStatus) => {
    if (item.localTitleId || item.adding) return
    setDiscoveryResults((prev) => prev.map((r) => (r.id === item.id ? { ...r, adding: true } : r)))
    try {
      const created = await apiFetch<Title>('/titles', {
        method: 'POST',
        body: JSON.stringify({
          type: item.type,
          is_anime: item.isAnime,
          year: item.year,
          status,
          match_status: 'confirmed',
          names: [{ name: item.title, language: 'en', is_primary: true }],
          cover_url: item.posterUrl,
          tmdb_id: item.tmdbId,
          anilist_id: item.anilistId,
        }),
      })

      setDiscoveryResults((prev) =>
        prev.map((r) => (r.id === item.id ? { ...r, localTitleId: created.id, adding: false } : r))
      )
      useTitleStore.getState().invalidate()

      showUndo({
        message: t('undo.titleAdded', { title: item.title }),
        onUndo: async () => {
          setDiscoveryResults((prev) =>
            prev.map((r) => (r.id === item.id ? { ...r, localTitleId: undefined } : r))
          )
          await apiFetch(`/titles/${created.id}`, { method: 'DELETE' })
          useTitleStore.getState().invalidate()
        },
      })
    } catch {
      setDiscoveryResults((prev) => prev.map((r) => (r.id === item.id ? { ...r, adding: false } : r)))
    }
  }

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
      route(routeTo.title(mergeTarget.id))
    } catch (e) {
      setMergeError('Merge failed. Please try again.')
      setMerging(false)
    }
  }

  const hasMatchedAlt = (t: Title) =>
    t.matched_name && t.matched_name !== getName(t)

  return (
    <PullToRefresh onRefresh={retry} disabled={filterOpen || Boolean(mergeTarget)}>
      <div className={s.page}>
        {/* Results area */}
        <div className={s.results}>
          {!query.trim() && (
            <div className={s.emptyState}>
              <div className={s.emptyInner}>
                <div className={s.emptyIcon}>
                  <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke={colors.inkDim} stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                    <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
                  </svg>
                </div>
                <div className={s.emptyText}>
                  {mergeSourceId ? t('search.emptyMerge') : t('search.emptyState')}
                </div>
                <div className={s.emptySubtext}>
                  {mergeSourceId
                    ? t('search.emptyMergeSub', { name: mergeSourceName ?? '' })
                    : t('search.emptyStateSub')}
                </div>
              </div>
            </div>
          )}

          {query.trim() && error && <ErrorBanner message={error} onRetry={retry} />}

          {results.length > 0 && (
            <>
              <div className={s.resultCount}>
                <span className={s.resultCountText}>
                  {total === 1
                    ? t('search.resultCountSingle', { query: query.trim() })
                    : t('search.resultCount', { count: results.length, total, query: query.trim() })}
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
                        route(routeTo.title(t.id))
                      }
                    }}
                    className={s.card}
                  >
                    <div className={s.coverWrap}>
                      <CoverImage
                        coverUrl={t.cover_url}
                        type={t.type}
                        is_anime={t.is_anime}
                        alt={getName(t)}
                        className={s.cardCover}
                        iconSize="18px"
                      />
                      <div className={s.typeBadge}>
                        <TypeBadge type={t.type} size="sm" radarrId={t.radarr_id} sonarrId={t.sonarr_id} />
                      </div>
                    </div>
                    <div className={s.cardBody}>
                      <div className={s.cardHeader}>
                        <span className={s.cardTitle}>{getName(t)}</span>
                        <StatusBadge status={t.status} caughtUp={t.caught_up} />
                        <ArrBadge type={t.type} radarrId={t.radarr_id} sonarrId={t.sonarr_id} />
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
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.inkDim} stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
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
                    {loadingMore ? t('search.loadingMore') : t('search.loadMore')}
                  </button>
                </div>
              )}
            </>
          )}

          {query.trim() && !loading && results.length === 0 && !error && (!searchOnTMDB || (!loadingDiscovery && discoveryItems.length === 0)) && (
            <div className={s.emptyPromptCard}>
              <p className={s.emptyPromptText}>
                {t('search.noResults', { query: query.trim() })}
              </p>
              {!searchOnTMDB ? (
                <div className={s.discoverPromptBox}>
                  <span className={s.discoverPromptSub}>{t('search.discoverPrompt')}</span>
                  <button
                    type="button"
                    className={s.discoverPromptBtn}
                    onClick={() => setSearchOnTMDB(true)}
                  >
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                      <circle cx="11" cy="11" r="8" />
                      <line x1="21" y1="21" x2="16.65" y2="16.65" />
                    </svg>
                    <span>{t('search.discoverAction')}</span>
                  </button>
                </div>
              ) : null}
            </div>
          )}

          {/* Discovery results when TMDB is active */}
          {searchOnTMDB && discoveryItems.length > 0 && (
            <>
              <div className={s.sectionDivider}>{t('search.tmdbResults')}</div>
              <div className={s.discoveryList}>
                {discoveryItems.map((r) => (
                  <div key={r.id} className={s.discoveryCard}>
                    <CoverImage
                      coverUrl={r.posterUrl}
                      type={r.type}
                      is_anime={r.isAnime}
                      alt={r.title}
                      className={s.cardCover}
                      iconSize="18px"
                    />
                    <div className={s.discoveryInfo}>
                      <div className={s.discoveryHeader}>
                        <span className={s.discoveryTitle}>{r.title}</span>
                        <span className={s.discoveryBadge}>{r.source}</span>
                      </div>
                      <div className={s.discoveryMeta}>
                        {getTypeLabel(r.type)} {r.year > 0 ? `· ${r.year}` : ''}
                      </div>
                    </div>
                    <div className={s.discoveryActions}>
                      {r.localTitleId ? (
                        <a
                          href={routeTo.title(r.localTitleId)}
                          className={s.inLibraryLink}
                          onClick={(e) => {
                            e.preventDefault()
                            route(routeTo.title(r.localTitleId!))
                          }}
                        >
                          {t('search.inLibrary')} ↗
                        </a>
                      ) : (
                        <>
                          <button
                            type="button"
                            disabled={r.adding}
                            onClick={() => handleAddDiscovery(r, 'plan_to_watch')}
                            className={s.btnAction}
                          >
                            {r.adding ? '...' : t('search.addToPlan')}
                          </button>
                          <button
                            type="button"
                            disabled={r.adding}
                            onClick={() => handleAddDiscovery(r, 'watching')}
                            className={clsx(s.btnAction, s.btnActionPrimary)}
                          >
                            {r.adding ? '...' : t('search.addToWatching')}
                          </button>
                        </>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            </>
          )}

          {query.trim() && (loading || loadingDiscovery) && (
            <div className={s.statusMessage}>
              {t('search.searching')}
            </div>
          )}
        </div>

        <BottomSheet open={!!mergeTarget} onClose={() => setMergeTarget(null)} ariaLabel="Merge titles">
          <div className={s.mergeDrawer}>
            <div className={s.mergeTitle}>{t('search.mergeTitle')}</div>
            <div className={s.mergeDesc}>
              {t('search.mergeDesc', {
                source: mergeSourceName ?? '',
                target: mergeTarget ? getName(mergeTarget) : '',
              })}
            </div>

            {mergeTarget?.type === 'series' && (
              <div className={s.seasonInputGroup}>
                <label htmlFor="target-season" className={s.seasonLabel}>{t('search.integrateAsSeason')}</label>
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

            {mergeError && <div className={s.mergeError}>{t('search.mergeError')}</div>}

            <div className={s.mergeActions}>
              <button className={s.cancelBtn} onClick={() => setMergeTarget(null)}>{t('common.cancel')}</button>
              <button
                className={s.confirmBtn}
                onClick={handleMerge}
                disabled={merging}
              >
                {merging ? t('search.merging') : t('search.mergeNow')}
              </button>
            </div>
          </div>
        </BottomSheet>
      </div>
    </PullToRefresh>
  )
}
