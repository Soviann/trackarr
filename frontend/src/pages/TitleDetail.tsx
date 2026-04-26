import { useState, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title } from '../types'
import { useApi } from '../hooks/useApi'
import { computeAniListUrl, getName, getTypeLabel, getStatusLabel, formatDate, formatDateTime, formatRelativeTime, formatWatchtime } from '../utils'
import { apiFetch } from '../api'
import { SeasonTab } from '../components/SeasonTab'
import { SeasonAniListStrip } from '../components/SeasonAniListStrip'
import { EpisodeRow } from '../components/EpisodeRow'
import { ActionDrawer } from '../components/ActionDrawer'
import { RatingPrompt } from '../components/RatingPrompt'
import { EditSheet } from '../components/EditSheet'
import { RematchSheet } from '../components/RematchSheet'
import { StatusBadge } from '../components/StatusBadge'
import { ErrorBanner } from '../components/ErrorBanner'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import { TitleHistory } from '../components/TitleHistory'
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
  const [activeSeason, setActiveSeason] = useState<number | null>(null)
  const [showRating, setShowRating] = useState(false)
  const [showEdit, setShowEdit] = useState(false)
  const [showRematch, setShowRematch] = useState(false)
  const [rematchSeasonID, setRematchSeasonID] = useState<number | null>(null)
  const [showHistory, setShowHistory] = useState(false)
  const [synopsisExpanded, setSynopsisExpanded] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

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
  const typeLabel = getTypeLabel(title.type)
  const current = sortedSeasons.find((ss) => ss.season_number === activeSeason)
    ?? sortedSeasons.find((ss) => (ss.episodes ?? []).some((e) => !e.watched))
    ?? sortedSeasons[sortedSeasons.length - 1]

  const currentEps = current?.episodes ?? []
  const watched = currentEps.filter((e) => e.watched).length
  const total = current?.total_episodes ?? currentEps.length
  const pct = total > 0 ? (watched / total) * 100 : 0

  const genres = parseJSON<string[]>(title.genres)
  const credits = parseJSON<{ name: string; role: string }[]>(title.credits)

  const handleRefresh = async () => {
    await apiFetch(`/titles/${title.id}/refresh`, { method: 'POST' })
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

  // Build meta line
  const metaParts = [typeLabel, String(title.year)]
  if (title.runtime) metaParts.push(formatRuntime(title.runtime))
  if (title.series_status) metaParts.push(formatSeriesStatus(title.series_status))

  return (
    <div className={s.page}>
      {actionError && <ErrorBanner message={actionError} onRetry={() => setActionError(null)} />}
      {/* Hero — pure visual */}
      <div
        className={s.hero}
        style={{
          background: title.cover_url
            ? `url(/api/covers/${title.cover_url}) center top/cover`
            : coverBackground(null, title.type),
        }}
      >
        {!title.cover_url && <CoverPlaceholder type={title.type} iconSize="48px" />}
        <div className={s.heroFade} />
        <button onClick={() => history.back()} aria-label="Back" className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
      </div>

      {/* Identity zone */}
      <div className={s.identity}>
        {title.cover_url ? (
          <img src={`/api/covers/${title.cover_url}`} alt="" role="presentation" className={s.miniPoster} width="80" height="120" />
        ) : (
          <div className={s.miniPosterPlaceholder} style={{ background: coverBackground(null, title.type) }}>
            <CoverPlaceholder type={title.type} iconSize="24px" />
          </div>
        )}
        <div className={s.identityInfo}>
          <div className={s.identityTitle}>{name}</div>
          <div className={s.identityMeta}>{metaParts.join(' · ')}</div>
          {genres && genres.length > 0 && (
            <div className={s.genrePills}>
              {genres.map((g) => <span key={g} className={s.genrePill}>{g}</span>)}
            </div>
          )}
          <div style={{ marginTop: '12px' }}>
            <StatusBadge status={title.status} />
          </div>
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
                  onClick={() => route('/person/' + encodeURIComponent(c.name))}
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
            <span className={s.detailVal}>{title.match_source}</span>
          </div>
        )}
        {title.original_title && title.original_title !== name && (
          <div className={s.detailRow}>
            <span className={s.detailKey}>Original title</span>
            <span className={s.detailVal}>{title.original_title}</span>
          </div>
        )}
      </div>

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
        </div>
      )}

      {/* Action drawer */}
      <ActionDrawer
        title={title}
        aniListUrl={computeAniListUrl(title)}
        onRate={() => setShowRating(true)}
        onEdit={() => setShowEdit(true)}
        onRematch={() => setShowRematch(true)}
        onMerge={() => route(`/search?mergeSourceId=${title.id}&mergeSourceName=${encodeURIComponent(name)}`)}
        onRefresh={handleRefresh}
      />

      {/* Bottom sheets */}
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
  )
}
