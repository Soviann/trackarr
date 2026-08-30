import { useState, useMemo } from 'preact/hooks'
import type { JSX } from 'preact'
import { route } from 'preact-router'
import type { Title, TitleRelation } from '../types'
import { useApi } from '../hooks/useApi'
import { computeAniListUrl, getName, getAlternativeNames, languageLabel, getTypeLabel, getStatusLabel, formatMatchSource, formatDate, formatDateTime, formatRelativeTime, formatWatchtime, hexToRgba } from '../utils'
import { apiFetch } from '../api'
import { colors } from '../theme'
import { SeasonTab } from '../components/SeasonTab'
import { SeasonAniListStrip } from '../components/SeasonAniListStrip'
import { EpisodeRow } from '../components/EpisodeRow'
import { ActionDrawer } from '../components/ActionDrawer'
import { RatingPrompt } from '../components/RatingPrompt'
import { EditSheet } from '../components/EditSheet'
import { RematchSheet } from '../components/RematchSheet'
import { ArrPushSheet } from '../components/ArrPushSheet'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import { StatusBadge } from '../components/StatusBadge'
import { PrimeBadge } from '../components/PrimeBadge'
import { isOnPrime } from '../utils/providers'
import { ErrorBanner } from '../components/ErrorBanner'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import { TitleHistory } from '../components/TitleHistory'
import { SeasonSideStories } from '../components/SeasonSideStories'
import { FranchiseRelationsSection } from '../components/FranchiseRelationsSection'
import { PullToRefresh } from '../components/PullToRefresh'
import { routeTo } from '../routes'
import { useTitleStore } from '../store'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import s from './TitleDetail.module.css'

function toggleEpisodeWatched(title: Title, episodeId: number): Title {
  return {
    ...title,
    seasons: title.seasons.map((s) => ({
      ...s,
      episodes: (s.episodes ?? []).map((ep) =>
        ep.id === episodeId ? { ...ep, watched: !ep.watched } : ep
      ),
    })),
  }
}

function formatSeriesStatus(st: string | null) {
  if (!st) return ''
  return st.charAt(0).toUpperCase() + st.slice(1).replace('_', ' ')
}

function formatRuntime(minutes: number): string {
  const h = Math.floor(minutes / 60)
  const m = minutes % 60
  return h > 0 ? `${h}h ${m.toString().padStart(2, '0')}m` : `${m}m`
}

function parseJSON<T>(json: string | null): T | null {
  if (!json) return null
  try { return JSON.parse(json) } catch { return null }
}

export function TitleDetail({ id }: { id?: string; path?: string }) {
  const { data: title, loading, error, mutate, setData } = useApi<Title>(id ? `/titles/${id}` : null)
  useScrollRestoration(`title-${id ?? ''}`, !loading && title !== null)
  const [activeSeason, setActiveSeason] = useState<number | null>(null)
  const [showRating, setShowRating] = useState(false)
  const [showEdit, setShowEdit] = useState(false)
  const [showRematch, setShowRematch] = useState(false)
  const [rematchSeasonID, setRematchSeasonID] = useState<number | null>(null)
  const [showArrPush, setShowArrPush] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [synopsisExpanded, setSynopsisExpanded] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const invalidate = useTitleStore((st) => st.invalidate)

  const sortedSeasons = useMemo(
    () => [...(title?.seasons ?? [])].sort((a, b) => a.season_number - b.season_number),
    [title?.id, title?.seasons]
  )

  if (loading || !title) {
    return (
      <div className={s.loading}>
        {error ? <ErrorBanner message={error} onRetry={mutate} /> : loading ? 'Loading...' : 'Title not found'}
      </div>
    )
  }

  const name = getName(title)
  const altNames = getAlternativeNames(title)
  const typeLabel = getTypeLabel(title.type)
  const current = sortedSeasons.find((ss) => ss.season_number === activeSeason)
    ?? sortedSeasons.find((ss) => (ss.episodes ?? []).some((e) => !e.watched))
    ?? sortedSeasons[sortedSeasons.length - 1]

  const currentEps = current?.episodes ?? []
  const watched = currentEps.filter((e) => e.watched).length
  const total = current?.total_episodes ?? currentEps.length
  const pct = total > 0 ? (watched / total) * 100 : 0

  const activeSeasonSideStories = useMemo(() => {
    if (!current || !title.relations) return []
    return title.relations.filter(
      (r) =>
        (r.season_id != null && r.season_id === current.id) ||
        (r.season_number != null && r.season_number === current.season_number)
    )
  }, [current?.id, current?.season_number, title.relations])

  const genres = title.genres
  const credits = parseJSON<{ name: string; role: string }[]>(title.credits)

  const handleRefresh = async () => {
    await apiFetch(`/titles/${title.id}/refresh`, { method: 'POST' })
  }

  // Pull-to-refresh: refetch the title so the spinner stays up until data lands.
  const handlePullRefresh = async () => {
    const updated = await apiFetch<Title>(`/titles/${title.id}`)
    setData(updated)
  }

  const handleEpisodeToggle = async (episodeId: number) => {
    setData((prev) => prev ? toggleEpisodeWatched(prev, episodeId) : prev)
    try {
      const updated = await apiFetch<Title>(`/titles/${title.id}/episodes/${episodeId}`, { method: 'PATCH' })
      setData(updated)
    } catch (e) {
      setActionError('Failed to update episode')
      mutate()
    }
  }

  const handleToggleSideStoryWatched = async (rel: TitleRelation) => {
    if (!rel.matched_title_id) return
    const isCompleted = rel.matched_status === 'completed'
    const newStatus = isCompleted ? 'plan_to_watch' : 'completed'
    try {
      await apiFetch(`/titles/${rel.matched_title_id}`, {
        method: 'PATCH',
        body: JSON.stringify({ status: newStatus }),
      })
      setData((prev) => {
        if (!prev) return prev
        return {
          ...prev,
          relations: (prev.relations ?? []).map((r) =>
            r.id === rel.id || r.external_id === rel.external_id
              ? { ...r, matched_status: newStatus }
              : r
          ),
        }
      })
    } catch (e) {
      setActionError('Failed to update title status')
    }
  }

  const handleSaveRating = async (rating: number) => {
    setShowRating(false)
    setData((prev) => prev ? { ...prev, my_rating: rating } : prev)
    try {
      const updated = await apiFetch<Title>(`/titles/${title.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ my_rating: rating }),
      })
      setData(updated)
    } catch (e) {
      setActionError('Failed to save rating')
      mutate()
    }
  }

  const handleSaveEdit = async (updates: { type?: string; status?: string }) => {
    setShowEdit(false)
    if (Object.keys(updates).length === 0) return
    try {
      const updated = await apiFetch<Title>(`/titles/${title.id}`, {
        method: 'PATCH',
        body: JSON.stringify(updates),
      })
      setData(updated)
    } catch (e) {
      setActionError('Failed to save changes')
      mutate()
    }
  }

  const handleArrPushSuccess = (arrId: number) => {
    setData((prev) => {
      if (!prev) return prev
      return prev.type === 'movie' ? { ...prev, radarr_id: arrId } : { ...prev, sonarr_id: arrId }
    })
  }

  // Deleting removes the whole title (seasons, episodes, history). We can't stay
  // on a page whose subject no longer exists, so route back to the library and
  // invalidate its cache so the deleted title drops out of the list.
  const handleDelete = async () => {
    try {
      await apiFetch(`/titles/${title.id}`, { method: 'DELETE' })
      invalidate()
      route(routeTo.home())
    } catch (e) {
      setActionError('Failed to delete title')
      throw e // keep the confirmation drawer open so the user can retry
    }
  }

  // Build meta line
  const metaParts = [typeLabel, String(title.year)]
  if (title.runtime) metaParts.push(formatRuntime(title.runtime))
  if (title.series_status) metaParts.push(formatSeriesStatus(title.series_status))

  const coverBg = coverBackground(title.cover_url, title.type)

  const pageStyle = {
    '--cover-bg': coverBg,
    ...(title.accent_hex && {
      '--cover-accent': title.accent_hex,
      '--cover-accent-wash': hexToRgba(title.accent_hex, 0.10),
    }),
  } as JSX.CSSProperties

  return (
    <PullToRefresh
      onRefresh={handlePullRefresh}
      disabled={drawerOpen || showRating || showEdit || showRematch || showHistory || showArrPush}
    >
    <div className={s.page} style={pageStyle}>
      {actionError && <ErrorBanner message={actionError} onRetry={() => setActionError(null)} />}

      {/* Hero — spacer, holds back button; cover image shows through .page background */}
      <div className={s.hero}>
        <button onClick={() => history.back()} aria-label="Back" className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={colors.ink} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
      </div>

      {/* Identity zone — title + meta float over the cover */}
      <div className={s.identity}>
        <div className={s.identityTitle}>{name}</div>
        <div className={s.identityMeta}>{metaParts.join(' · ')}</div>
        {genres && genres.length > 0 && (
          <div className={s.genrePills}>
            {genres.map((g) => <span key={g} className={s.genrePill}>{g}</span>)}
          </div>
        )}
        <div style={{ marginTop: '12px', display: 'flex', gap: '6px', alignItems: 'center' }}>
          <StatusBadge status={title.status} />
          {isOnPrime(title.watch_providers) && <PrimeBadge />}
        </div>
      </div>

      {/* Ratings card */}
      <div className={s.card} style={{ marginTop: '12px' }}>
        <div className={s.ratingsRow}>
          <div>
            <div className={s.cardLabel}>My rating</div>
            {title.my_rating != null ? (
              <div className={s.myRating}>{title.my_rating}<span className={s.myRatingSuffix}>/10</span></div>
            ) : (
              <div className={s.noRating}>Not rated</div>
            )}
          </div>
          <div className={s.extRatings}>
            {title.tmdb_rating != null && (
              <div className={s.extItem}>
                <div className={`${s.extScore} ${s.tmdbColor}`}>{title.tmdb_rating.toFixed(1)}</div>
                <div className={s.extSource}>TMDB</div>
              </div>
            )}
            {title.anilist_rating != null && (
              <div className={s.extItem}>
                <div className={`${s.extScore} ${s.anilistColor}`}>{title.anilist_rating}%</div>
                <div className={s.extSource}>AniList</div>
              </div>
            )}

          </div>
        </div>
      </div>

      {/* Synopsis card */}
      {title.overview && (
        <div className={s.card}>
          <div className={s.cardLabel}>Synopsis</div>
          <div className={`${s.synopsisText} ${!synopsisExpanded ? s.synopsisClamped : ''}`}>
            {title.overview}
          </div>
          <button className={s.synopsisToggle} onClick={() => setSynopsisExpanded(!synopsisExpanded)}>
            {synopsisExpanded ? 'Show less' : 'Show more'}
          </button>
        </div>
      )}

      {/* Cast & Crew card */}
      {credits && credits.length > 0 && (
        <div className={s.card}>
          <div className={s.cardLabel}>Cast & Crew</div>
          <div className={s.castList}>
            {credits.map((c) => (
              <div key={`${c.name}-${c.role}`} className={s.castEntry}>
                <button
                  type="button"
                  className={s.castPerson}
                  onClick={() => route(routeTo.person(c.name))}
                >
                  {c.name}
                </button>
                <span className={s.castRole}>{c.role}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Details card */}
      <div className={s.card}>
        <div className={s.cardLabel}>Details</div>
        <div className={s.detailRow}>
          <span className={s.detailKey}>Added</span>
          <span className={s.detailVal}>{formatDate(title.created_at)}</span>
        </div>
        {title.last_watched_at && (
          <div className={s.detailRow}>
            <span className={s.detailKey}>Last watched</span>
            <span className={s.detailVal}>{formatDate(title.last_watched_at)}</span>
          </div>
        )}
        {formatWatchtime(title.total_watch_minutes) && (
          <div className={s.detailRow}>
            <span className={s.detailKey}>Watch time</span>
            <span className={s.detailVal}>{formatWatchtime(title.total_watch_minutes)}</span>
          </div>
        )}
        <div className={s.detailRow}>
          <span className={s.detailKey}>Last refreshed</span>
          <span
            className={s.detailVal}
            title={title.last_refreshed_at ? formatDateTime(title.last_refreshed_at) : undefined}
          >
            {title.last_refreshed_at ? formatRelativeTime(title.last_refreshed_at) : 'Never'}
          </span>
        </div>
        {title.match_source && (
          <div className={s.detailRow}>
            <span className={s.detailKey}>Match</span>
            <span className={s.detailVal}>{formatMatchSource(title.match_source)}</span>
          </div>
        )}
        {(title.imdb_id || (title.tmdb_id != null && title.tmdb_id > 0) || (title.tvdb_id != null && title.tvdb_id > 0) || computeAniListUrl(title)) && (
          <div className={s.detailRow}>
            <span className={s.detailKey}>Sources</span>
            <div className={s.externalLinksWrap}>
              {title.imdb_id && (
                <a
                  href={`https://www.imdb.com/title/${title.imdb_id}/`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`${s.extLinkBadge} ${s.extLinkImdb}`}
                >
                  IMDb
                </a>
              )}
              {title.tmdb_id != null && title.tmdb_id > 0 && (
                <a
                  href={`https://www.themoviedb.org/${title.type === 'movie' ? 'movie' : 'tv'}/${title.tmdb_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`${s.extLinkBadge} ${s.extLinkTmdb}`}
                >
                  TMDB
                </a>
              )}
              {title.tvdb_id != null && title.tvdb_id > 0 && (
                <a
                  href={`https://thetvdb.com/dereferrer/${title.type === 'movie' ? 'movie' : 'series'}/${title.tvdb_id}`}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`${s.extLinkBadge} ${s.extLinkTvdb}`}
                >
                  TVDB
                </a>
              )}
              {computeAniListUrl(title) && (
                <a
                  href={computeAniListUrl(title)!}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={`${s.extLinkBadge} ${s.extLinkAnilist}`}
                >
                  AniList
                </a>
              )}
            </div>
          </div>
        )}
        <div className={s.detailRow}>
          <span className={s.detailKey}>{title.type === 'movie' ? 'Radarr' : 'Sonarr'}</span>
          <span className={s.detailVal}>
            <button
              type="button"
              onClick={() => setShowArrPush(true)}
              className={`${s.arrAddBtn} ${title.type === 'movie' ? s.radarrBtn : s.sonarrBtn}`}
            >
              {title.radarr_id != null || title.sonarr_id != null ? (
                <>
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="12" cy="12" r="3" />
                    <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
                  </svg>
                  Gérer dans {title.type === 'movie' ? 'Radarr' : 'Sonarr'}
                </>
              ) : (
                <>
                  <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <line x1="22" y1="2" x2="11" y2="13" />
                    <polygon points="22 2 15 22 11 13 2 9 22 2" />
                  </svg>
                  Envoyer à {title.type === 'movie' ? 'Radarr' : 'Sonarr'}
                </>
              )}
            </button>
          </span>
        </div>
        {title.original_title && title.original_title !== name && (
          <div className={s.detailRow}>
            <span className={s.detailKey}>Original title</span>
            <span className={s.detailVal}>{title.original_title}</span>
          </div>
        )}
        {altNames.length > 0 && (
          <div className={s.altNames}>
            <div className={s.altNamesLabel}>Autres titres</div>
            {altNames.map((alt) => {
              const lang = languageLabel(alt.language)
              return (
                <div key={`${alt.language}-${alt.name}`} className={s.altNameRow}>
                  <span className={s.altNameFlag} title={lang.label}>{lang.flag}</span>
                  <span className={s.altNameText}>{alt.name}</span>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Franchise & Univers Relations */}
      {title.relations && title.relations.length > 0 && (
        <FranchiseRelationsSection relations={title.relations} />
      )}

      {/* Historique button */}
      <div className={s.historyBtnWrap}>
        <button className={s.historyBtn} onClick={() => setShowHistory(true)}>
          Historique
        </button>
      </div>

      {/* Watch history overlay */}
      {showHistory && (
        <div className={s.historyOverlay}>
          <TitleHistory titleId={title.id} onClose={() => setShowHistory(false)} />
        </div>
      )}

      {/* Progress bar (series/anime) */}
      {current && title.type !== 'movie' && (
        <div className={s.progressWrap}>
          <div className={s.progressTrack}>
            <div className={s.progressBar} style={{ width: `${pct}%` }} />
          </div>
          <div className={s.progressLabel}>
            S{current.season_number} · {watched} of {total} episodes watched
          </div>
        </div>
      )}

      {/* Season tabs */}
      {sortedSeasons.length > 1 && (
        <div className={s.seasonTabs}>
          {sortedSeasons.map((ss) => (
            <SeasonTab
              key={ss.id}
              season={ss}
              active={ss.id === current?.id}
              onClick={() => setActiveSeason(ss.season_number)}
            />
          ))}
        </div>
      )}

      {/* AniList strip for the active season */}
      {current && title.type !== 'movie' && title.is_anime && (
        <SeasonAniListStrip
          season={current}
          onEdit={() => setRematchSeasonID(current.id)}
        />
      )}

      {/* Episode list */}
      {current && (
        <div className={s.episodeList}>
          {[...(current.episodes ?? [])]
            .sort((a, b) => a.episode - b.episode)
            .map((ep) => (
              <EpisodeRow key={ep.id} episode={ep} onToggle={handleEpisodeToggle} />
            ))}

          {/* Side stories for the active season */}
          {activeSeasonSideStories.length > 0 && (
            <SeasonSideStories
              seasonNumber={current.season_number}
              sideStories={activeSeasonSideStories}
              onToggleWatched={handleToggleSideStoryWatched}
            />
          )}
        </div>
      )}

      {/* Action drawer */}
      <ActionDrawer
        title={title}
        onRate={() => setShowRating(true)}
        onEdit={() => setShowEdit(true)}
        onRematch={() => setShowRematch(true)}
        onMerge={() => route(`/search?mergeSourceId=${title.id}&mergeSourceName=${encodeURIComponent(name)}`)}
        onRefresh={handleRefresh}
        onDelete={() => setShowDeleteConfirm(true)}
        onOpenChange={setDrawerOpen}
      />

      <ConfirmationDrawer
        open={showDeleteConfirm}
        onClose={() => setShowDeleteConfirm(false)}
        onConfirm={handleDelete}
        title={`Delete "${name}"?`}
        description="This removes the title and all its watch history. This cannot be undone."
        confirmText="Delete"
        isDangerous
      />

      {/* Bottom sheets */}
      <ArrPushSheet
        open={showArrPush}
        onClose={() => setShowArrPush(false)}
        title={title}
        onSuccess={handleArrPushSuccess}
      />

      <RatingPrompt
        open={showRating}
        onClose={() => setShowRating(false)}
        titleName={name}
        initialRating={title.my_rating}
        hasImdb={!!title.imdb_id}
        onSave={handleSaveRating}
        onSaveAndImdb={(rating) => {
          handleSaveRating(rating)
          if (title.imdb_id) window.open(`https://www.imdb.com/title/${title.imdb_id}/`, '_blank', 'noopener,noreferrer')
        }}
      />

      <EditSheet
        open={showEdit}
        onClose={() => setShowEdit(false)}
        title={title}
        onSave={handleSaveEdit}
      />


      <RematchSheet
        open={showRematch || rematchSeasonID != null}
        onClose={() => { setShowRematch(false); setRematchSeasonID(null) }}
        title={title}
        seasonID={rematchSeasonID ?? undefined}
        onDone={() => { setRematchSeasonID(null); mutate() }}
      />
    </div>
    </PullToRefresh>
  )
}
