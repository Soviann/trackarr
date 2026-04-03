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
  const currentSeason = title.seasons
    .filter((s) => s.episodes.length > 0)
    .sort((a, b) => b.season_number - a.season_number)
    .find((s) => s.episodes.some((e) => e.watched))

  if (!currentSeason) {
    const first = title.seasons[0]
    return { season: first, watched: 0, total: first?.total_episodes ?? 0, nextEpisode: first?.episodes[0] ?? null }
  }

  const watched = currentSeason.episodes.filter((e) => e.watched).length
  const total = currentSeason.total_episodes ?? currentSeason.episodes.length
  const nextEpisode = currentSeason.episodes
    .sort((a, b) => a.episode - b.episode)
    .find((e) => !e.watched) ?? null

  return { season: currentSeason, watched, total, nextEpisode }
}

export function TitleCard({ title, onUpdate }: TitleCardProps) {
  const [toggling, setToggling] = useState(false)
  const name = title.names.find((n) => n.is_primary)?.name ?? 'Untitled'
  const typeLabel = title.type.charAt(0).toUpperCase() + title.type.slice(1)

  const { season, watched, total, nextEpisode } = getProgress(title)
  const pct = total > 0 ? (watched / total) * 100 : 0

  const handleQuickMark = async (e: Event) => {
    e.stopPropagation()
    if (!nextEpisode || toggling) return
    setToggling(true)
    try {
      await apiFetch(`/titles/${title.id}/episodes/${nextEpisode.id}`, { method: 'PATCH' })
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
      {nextEpisode && (
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
            fontSize: nextEpisode.episode >= 10 ? '10px' : '11px',
            fontWeight: 700,
            color: colors.bgPrimary,
          }}>
            E{nextEpisode.episode}
          </span>
        </div>
      )}
    </div>
  )
}
