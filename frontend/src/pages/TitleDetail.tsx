import { useState } from 'preact/hooks'
import type { Title } from '../types'
import { useApi } from '../hooks/useApi'
import { getName, getTypeLabel, getStatusLabel } from '../utils'
import { apiFetch } from '../api'
import { SeasonTab } from '../components/SeasonTab'
import { EpisodeRow } from '../components/EpisodeRow'
import { ActionDrawer } from '../components/ActionDrawer'
import { RatingPrompt } from '../components/RatingPrompt'
import { EditSheet } from '../components/EditSheet'
import { RematchSheet } from '../components/RematchSheet'
import { AniListSheet } from '../components/AniListSheet'
import { ErrorBanner } from '../components/ErrorBanner'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import s from './TitleDetail.module.css'

function getNextUnwatched(title: Title) {
  for (const season of [...(title.seasons ?? [])].sort((a, b) => a.season_number - b.season_number)) {
    for (const ep of [...(season.episodes ?? [])].sort((a, b) => a.episode - b.episode)) {
      if (!ep.watched) return { season, episode: ep }
    }
  }
  return null
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

function formatDate(dateStr: string | null | undefined): string {
  if (!dateStr) return ''
  return new Date(dateStr).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}

export function TitleDetail({ id }: { id?: string; path?: string }) {
  const { data: title, loading, error, mutate } = useApi<Title>(id ? `/titles/${id}` : null)
  const [activeSeason, setActiveSeason] = useState<number | null>(null)
  const [showRating, setShowRating] = useState(false)
  const [showEdit, setShowEdit] = useState(false)
  const [showAniList, setShowAniList] = useState(false)
  const [showRematch, setShowRematch] = useState(false)
  const [synopsisExpanded, setSynopsisExpanded] = useState(false)

  if (loading || !title) {
    return (
      <div className={s.loading}>
        {error ? <ErrorBanner message={error} onRetry={mutate} /> : loading ? 'Loading...' : 'Title not found'}
      </div>
    )
  }

  const name = getName(title)
  const typeLabel = getTypeLabel(title.type)
  const sortedSeasons = [...(title.seasons ?? [])].sort((a, b) => a.season_number - b.season_number)
  const current = sortedSeasons.find((ss) => ss.season_number === activeSeason)
    ?? sortedSeasons.find((ss) => (ss.episodes ?? []).some((e) => !e.watched))
    ?? sortedSeasons[sortedSeasons.length - 1]

  const currentEps = current?.episodes ?? []
  const watched = currentEps.filter((e) => e.watched).length
  const total = current?.total_episodes ?? currentEps.length
  const pct = total > 0 ? (watched / total) * 100 : 0
  const next = getNextUnwatched(title)

  const genres = parseJSON<string[]>(title.genres)
  const credits = parseJSON<{ name: string; role: string }[]>(title.credits)

  const handleMarkNext = async () => {
    if (!next) return
    await apiFetch(`/titles/${title.id}/episodes/${next.episode.id}`, { method: 'PATCH' })
    mutate()
  }

  const handleSaveRating = async (rating: number) => {
    await apiFetch(`/titles/${title.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ my_rating: rating }),
    })
    setShowRating(false)
    mutate()
  }

  const handleSaveEdit = async (updates: { type?: string; status?: string }) => {
    if (Object.keys(updates).length > 0) {
      await apiFetch(`/titles/${title.id}`, {
        method: 'PATCH',
        body: JSON.stringify(updates),
      })
      mutate()
    }
    setShowEdit(false)
  }

  const handleConfirmAniList = async () => {
    await apiFetch(`/titles/${title.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ match_status: 'confirmed' }),
    })
    setShowAniList(false)
    mutate()
  }

  // Build meta line
  const metaParts = [typeLabel, String(title.year)]
  if (title.runtime) metaParts.push(formatRuntime(title.runtime))
  if (title.series_status) metaParts.push(formatSeriesStatus(title.series_status))

  return (
    <div className={s.page}>
      {/* Hero — pure visual */}
      <div
        className={s.hero}
        style={{ background: coverBackground(title.cover_url, title.type) }}
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
          <img src={`/api/covers/${title.cover_url}`} alt="" className={s.miniPoster} />
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
            {credits.map((c, i) => (
              <div key={i} className={s.castEntry}>
                <span className={s.castPerson}>{c.name}</span>
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
          <span className={s.detailKey}>Status</span>
          <span className={s.detailVal}>{getStatusLabel(title.status)}</span>
        </div>
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

      {/* Episode list */}
      {current && (
        <div className={s.episodeList}>
          {[...(current.episodes ?? [])]
            .sort((a, b) => a.episode - b.episode)
            .map((ep) => (
              <EpisodeRow key={ep.id} titleId={title.id} episode={ep} onToggle={mutate} />
            ))}
        </div>
      )}

      {/* Action drawer */}
      <ActionDrawer
        title={title}
        nextEpisode={next?.episode ?? null}
        nextSeasonNumber={next?.season.season_number}
        onMarkNext={handleMarkNext}
        onRate={() => setShowRating(true)}
        onEdit={() => setShowEdit(true)}
        onRematch={() => setShowRematch(true)}
        onAniList={() => setShowAniList(true)}
      />

      {/* Bottom sheets */}
      <RatingPrompt
        open={showRating}
        onClose={() => setShowRating(false)}
        titleName={name}
        initialRating={title.my_rating}
        hasImdb={!!title.imdb_id}
        hasAnilist={title.is_anime}
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

      {title.is_anime && (
        <AniListSheet
          open={showAniList}
          onClose={() => setShowAniList(false)}
          title={title}
          onConfirm={handleConfirmAniList}
        />
      )}

      <RematchSheet
        open={showRematch}
        onClose={() => setShowRematch(false)}
        title={title}
        onDone={mutate}
      />
    </div>
  )
}
