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
  const currentSeason = seasons
    .filter((s) => (s.episodes ?? []).length > 0)
    .sort((a, b) => b.season_number - a.season_number)
    .find((s) => (s.episodes ?? []).some((e) => e.watched))

  if (!currentSeason) {
    const first = seasons[0]
    const eps = first?.episodes ?? []
    return { season: first, watched: 0, total: first?.total_episodes ?? 0, nextEpisode: eps[0] ?? null }
  }

  const eps = currentSeason.episodes ?? []
  const watched = eps.filter((e) => e.watched).length
  const total = currentSeason.total_episodes ?? eps.length
  const nextEpisode = eps
    .sort((a, b) => a.episode - b.episode)
    .find((e) => !e.watched) ?? null

  return { season: currentSeason, watched, total, nextEpisode }
}

export function TitleCard({ title, onUpdate }: TitleCardProps) {
  const [toggling, setToggling] = useState(false)
  const name = (title.names ?? []).find((n) => n.is_primary)?.name ?? 'Untitled'
  const typeLabel = title.type.charAt(0).toUpperCase() + title.type.slice(1)

  const progress = title.type !== 'movie' ? getProgress(title) : null
  const season = progress?.season
  const watched = progress?.watched ?? 0
  const total = progress?.total ?? 0
  const nextEpisode = progress?.nextEpisode ?? null
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
