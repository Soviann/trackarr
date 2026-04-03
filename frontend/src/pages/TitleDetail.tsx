import { useState } from 'preact/hooks'
import type { Title } from '../types'
import { colors } from '../theme'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { SeasonTab } from '../components/SeasonTab'
import { EpisodeRow } from '../components/EpisodeRow'
import { ActionBar } from '../components/ActionBar'
import { RatingPrompt } from '../components/RatingPrompt'
import { EditSheet } from '../components/EditSheet'
import { AniListSheet } from '../components/AniListSheet'

function getNextUnwatched(title: Title) {
  for (const season of [...title.seasons].sort((a, b) => a.season_number - b.season_number)) {
    for (const ep of [...season.episodes].sort((a, b) => a.episode - b.episode)) {
      if (!ep.watched) return { season, episode: ep }
    }
  }
  return null
}

function formatSeriesStatus(s: string | null) {
  if (!s) return ''
  return s.charAt(0).toUpperCase() + s.slice(1).replace('_', ' ')
}

export function TitleDetail({ id }: { id?: string; path?: string }) {
  const { data: title, loading, mutate } = useApi<Title>(id ? `/titles/${id}` : null)
  const [activeSeason, setActiveSeason] = useState<number | null>(null)
  const [showRating, setShowRating] = useState(false)
  const [showEdit, setShowEdit] = useState(false)
  const [showAniList, setShowAniList] = useState(false)

  if (loading || !title) {
    return (
      <div style={{ padding: '40px 16px', textAlign: 'center', color: colors.textSecondary }}>
        {loading ? 'Loading...' : 'Title not found'}
      </div>
    )
  }

  const name = title.names.find((n) => n.is_primary)?.name ?? 'Untitled'
  const typeLabel = title.type.charAt(0).toUpperCase() + title.type.slice(1)
  const sortedSeasons = [...title.seasons].sort((a, b) => a.season_number - b.season_number)
  const current = sortedSeasons.find((s) => s.season_number === activeSeason)
    ?? sortedSeasons.find((s) => s.episodes.some((e) => !e.watched))
    ?? sortedSeasons[sortedSeasons.length - 1]

  const watched = current?.episodes.filter((e) => e.watched).length ?? 0
  const total = current?.total_episodes ?? current?.episodes.length ?? 0
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
    <div style={{ paddingBottom: '36px' }}>
      {/* Hero cover */}
      <div style={{
        position: 'relative',
        height: '160px',
        background: title.cover_url
          ? `url(/api/covers/${title.cover_url}) center/cover`
          : `linear-gradient(135deg, ${colors.bgSurface}, ${colors.bgCard})`,
        display: 'flex',
        alignItems: 'flex-end',
      }}>
        {/* Back button */}
        <div
          onClick={() => history.back()}
          style={{
            position: 'absolute',
            top: '14px',
            left: '14px',
            width: '32px',
            height: '32px',
            background: 'rgba(0,0,0,0.5)',
            borderRadius: '50%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
          }}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </div>

        {/* Top-right buttons */}
        <div style={{ position: 'absolute', top: '14px', right: '14px', display: 'flex', gap: '8px' }}>
          {title.type === 'anime' && (
            <div
              onClick={() => setShowAniList(true)}
              style={{
                width: '32px',
                height: '32px',
                background: 'rgba(0,0,0,0.5)',
                borderRadius: '50%',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                position: 'relative',
                cursor: 'pointer',
              }}
            >
              <span style={{ fontSize: '10px', fontWeight: 700, color: colors.accentAnilist }}>AL</span>
              {title.match_status === 'pending_review' && (
                <div style={{
                  position: 'absolute', top: 0, right: 0,
                  width: '8px', height: '8px', borderRadius: '50%',
                  background: colors.accentAmber, border: '1.5px solid rgba(0,0,0,0.5)',
                }} />
              )}
            </div>
          )}
          {/* Edit button */}
          <div
            onClick={() => setShowEdit(true)}
            style={{
              width: '32px',
              height: '32px',
              background: 'rgba(0,0,0,0.5)',
              borderRadius: '50%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              cursor: 'pointer',
            }}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
            </svg>
          </div>
        </div>

        {/* Title info over gradient */}
        <div style={{
          width: '100%',
          padding: '14px 16px',
          background: 'linear-gradient(transparent, #0D0D0D)',
        }}>
          <div style={{ fontSize: '20px', fontWeight: 700, color: '#fff' }}>{name}</div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '3px' }}>
            <span style={{ fontSize: '11px', color: '#aaa' }}>
              {typeLabel} · {title.year}
              {title.series_status && ` · ${formatSeriesStatus(title.series_status)}`}
            </span>
            {title.my_rating != null && (
              <span style={{ fontSize: '12px', fontWeight: 600, color: colors.accentAmber }}>
                ★ {title.my_rating}/10
              </span>
            )}
          </div>
        </div>
      </div>

      {/* Progress bar */}
      {current && title.type !== 'movie' && (
        <div style={{ padding: '12px 16px 10px' }}>
          <div style={{ height: '3px', background: '#2A2A2A', borderRadius: '2px', overflow: 'hidden' }}>
            <div style={{ width: `${pct}%`, height: '100%', background: colors.accentAmber, borderRadius: '2px' }} />
          </div>
          <div style={{ fontSize: '10px', color: colors.textSecondary, marginTop: '4px' }}>
            S{current.season_number} · {watched} of {total} episodes watched
          </div>
        </div>
      )}

      {/* Season tabs */}
      {sortedSeasons.length > 1 && (
        <div style={{ padding: '0 16px 10px', display: 'flex', gap: '8px', overflowX: 'auto' }}>
          {sortedSeasons.map((s) => (
            <SeasonTab
              key={s.id}
              season={s}
              active={s.id === current?.id}
              onClick={() => setActiveSeason(s.season_number)}
            />
          ))}
        </div>
      )}

      {/* Episode list */}
      {current && (
        <div style={{ padding: '0 16px', display: 'flex', flexDirection: 'column', gap: '6px' }}>
          {[...current.episodes]
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
          if (title.imdb_id) window.open(`https://www.imdb.com/title/${title.imdb_id}/`, '_blank')
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
    </div>
  )
}
