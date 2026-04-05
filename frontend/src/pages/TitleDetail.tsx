import { useState } from 'preact/hooks'
import type { Title } from '../types'
import { colors } from '../theme'
import { useApi } from '../hooks/useApi'
import { getName, getTypeLabel } from '../utils'
import { apiFetch } from '../api'
import { SeasonTab } from '../components/SeasonTab'
import { EpisodeRow } from '../components/EpisodeRow'
import { ActionBar } from '../components/ActionBar'
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

export function TitleDetail({ id }: { id?: string; path?: string }) {
  const { data: title, loading, error, mutate } = useApi<Title>(id ? `/titles/${id}` : null)
  const [activeSeason, setActiveSeason] = useState<number | null>(null)
  const [showRating, setShowRating] = useState(false)
  const [showEdit, setShowEdit] = useState(false)
  const [showAniList, setShowAniList] = useState(false)
  const [showRematch, setShowRematch] = useState(false)

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

  return (
    <div className={s.page}>
      {/* Hero cover */}
      <div
        className={s.hero}
        style={{ background: coverBackground(title.cover_url, title.type) }}
      >
        {!title.cover_url && <CoverPlaceholder type={title.type} iconSize="48px" />}
        {/* Back button */}
        <button onClick={() => history.back()} aria-label="Retour" className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>

        {/* Top-right buttons */}
        <div className={s.topRight}>
          {title.type === 'anime' && (
            <button onClick={() => setShowAniList(true)} aria-label="AniList" className={s.anilistBtn}>
              <span className={s.anilistLabel}>AL</span>
              {title.match_status === 'pending_review' && <div className={s.pendingDot} />}
            </button>
          )}
          {/* Fix match button */}
          <button onClick={() => setShowRematch(true)} aria-label="Fix match" className={s.overlayBtn}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
          </button>
          {/* Edit button */}
          <button onClick={() => setShowEdit(true)} aria-label="Modifier" className={s.overlayBtn}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
            </svg>
          </button>
        </div>

        {/* Title info over gradient */}
        <div className={s.heroInfo}>
          <div className={s.heroTitle}>{name}</div>
          <div className={s.heroMeta}>
            <span className={s.heroSubtitle}>
              {typeLabel} · {title.year}
              {title.series_status && ` · ${formatSeriesStatus(title.series_status)}`}
            </span>
            {title.my_rating != null && (
              <span className={s.heroRating}>
                ★ {title.my_rating}/10
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Progress bar */}
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

      {/* Action bar */}
      <ActionBar
        title={title}
        nextEpisode={next?.episode ?? null}
        nextSeasonNumber={next?.season.season_number}
        onMarkNext={handleMarkNext}
        onRate={() => setShowRating(true)}
        onAniList={() => setShowAniList(true)}
      />

      {/* Bottom sheets */}
      <RatingPrompt
        open={showRating}
        onClose={() => setShowRating(false)}
        titleName={name}
        initialRating={title.my_rating}
        hasImdb={!!title.imdb_id}
        hasAnilist={title.type === 'anime'}
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

      {title.type === 'anime' && (
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
