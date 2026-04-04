import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title } from '../types'
import { colors } from '../theme'
import { apiFetch } from '../api'

interface TitleCardProps {
  title: Title
  onUpdate?: () => void
}

function getProgress(title: Title) {
  const seasons = title.seasons ?? []
  const ne = title.next_episode

  // Find the relevant season: the one with the next unwatched episode, or the latest with episodes
  let currentSeason = ne
    ? seasons.find((s) => s.season_number === ne.season_number)
    : null
  if (!currentSeason) {
    currentSeason = seasons
      .filter((s) => (s.episode_count ?? (s.episodes ?? []).length) > 0)
      .sort((a, b) => b.season_number - a.season_number)[0] ?? seasons[0]
  }
  if (!currentSeason) return null

  const watched = currentSeason.watched_count ?? (currentSeason.episodes ?? []).filter((e) => e.watched).length
  const total = currentSeason.total_episodes ?? currentSeason.episode_count ?? (currentSeason.episodes ?? []).length

  return { season: currentSeason, watched, total }
}

export function TitleCard({ title, onUpdate }: TitleCardProps) {
  const [toggling, setToggling] = useState(false)
  const name = (title.names ?? []).find((n) => n.is_primary)?.name ?? 'Untitled'
  const typeLabel = title.type.charAt(0).toUpperCase() + title.type.slice(1)

  const progress = title.type !== 'movie' ? getProgress(title) : null
  const season = progress?.season
  const watched = progress?.watched ?? 0
  const total = progress?.total ?? 0
  const ne = title.next_episode ?? null
  const pct = total > 0 ? (watched / total) * 100 : 0

  const handleQuickMark = async (e: Event) => {
    e.stopPropagation()
    if (!ne || toggling) return
    setToggling(true)
    try {
      await apiFetch(`/titles/${title.id}/episodes/${ne.id}`, { method: 'PATCH' })
      onUpdate?.()
    } finally {
      setToggling(false)
    }
  }

  return (
    <div
      onClick={() => route(`/title/${title.id}`)}
      style={{
        display: 'flex',
        gap: '12px',
        alignItems: 'center',
        background: colors.bgCard,
        borderRadius: '12px',
        padding: '10px',
        border: `1px solid ${colors.borderCard}`,
        cursor: 'pointer',
      }}
    >
      {/* Cover */}
      <div style={{
        width: '48px',
        height: '68px',
        borderRadius: '6px',
        flexShrink: 0,
        background: title.cover_url
          ? `url(/api/covers/${title.cover_url}) center/cover`
          : `linear-gradient(135deg, ${colors.bgSurface}, ${colors.bgCard})`,
      }} />

      {/* Info */}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{
          fontSize: '13px',
          fontWeight: 600,
          color: colors.textPrimary,
          overflow: 'hidden',
          textOverflow: 'ellipsis',
          whiteSpace: 'nowrap',
        }}>
          {name}
        </div>
        <div style={{ fontSize: '10px', color: colors.textSecondary, marginTop: '2px' }}>
          {typeLabel} · {title.year}
        </div>
        {season && (
          <>
            <div style={{
              marginTop: '6px',
              height: '3px',
              background: '#2A2A2A',
              borderRadius: '2px',
              overflow: 'hidden',
            }}>
              <div style={{
                width: `${pct}%`,
                height: '100%',
                background: colors.accentAmber,
                borderRadius: '2px',
              }} />
            </div>
            <div style={{ fontSize: '9px', color: colors.textSecondary, marginTop: '2px' }}>
              S{season.season_number} · {watched}/{total}
            </div>
          </>
        )}
      </div>

      {/* Quick mark badge */}
      {ne && (
        <div
          onClick={handleQuickMark}
          style={{
            width: '34px',
            height: '34px',
            borderRadius: '50%',
            background: toggling ? colors.textMuted : colors.accentAmber,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            flexShrink: 0,
            cursor: 'pointer',
          }}
        >
          <span style={{
            fontSize: ne.episode >= 10 ? '10px' : '11px',
            fontWeight: 700,
            color: colors.bgPrimary,
          }}>
            E{ne.episode}
          </span>
        </div>
      )}
    </div>
  )
}
