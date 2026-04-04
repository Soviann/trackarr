import { useState } from 'preact/hooks'
import { colors } from '../theme'
import { apiFetch } from '../api'
import type { Episode } from '../types'

interface EpisodeRowProps {
  titleId: number
  episode: Episode
  onToggle?: () => void
}

export function EpisodeRow({ titleId, episode, onToggle }: EpisodeRowProps) {
  const [toggling, setToggling] = useState(false)

  const handleToggle = async (e: Event) => {
    e.stopPropagation()
    if (toggling) return
    setToggling(true)
    try {
      await apiFetch(`/titles/${titleId}/episodes/${episode.id}`, { method: 'PATCH' })
      onToggle?.()
    } finally {
      setToggling(false)
    }
  }

  return (
    <div style={{
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      padding: '8px 12px',
      background: colors.bgCard,
      borderRadius: '8px',
      border: `1px solid ${episode.watched ? colors.borderCard : colors.bgSurface}`,
    }}>
      <div style={{ flex: 1, minWidth: 0, display: 'flex', alignItems: 'baseline', gap: '6px' }}>
        <span style={{ fontSize: '12px', color: colors.textPrimary, flexShrink: 0 }}>E{episode.episode}</span>
        {episode.name && (
          <span style={{
            fontSize: '11px',
            color: colors.textMuted,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            — {episode.name}
          </span>
        )}
        {episode.air_date && (
          <span style={{ fontSize: '10px', color: colors.textDimmed, flexShrink: 0, marginLeft: 'auto' }}>
            {episode.air_date}
          </span>
        )}
      </div>

      <div onClick={handleToggle} style={{ cursor: 'pointer', flexShrink: 0, marginLeft: '8px' }}>
        {episode.watched ? (
          <svg width="18" height="18" viewBox="0 0 24 24" fill={colors.accentAmber} stroke="none">
            <path d="M20 6L9 17l-5-5 1.41-1.41L9 14.17 18.59 4.58z" />
          </svg>
        ) : (
          <div style={{
            width: '18px',
            height: '18px',
            borderRadius: '4px',
            border: `1.5px solid ${colors.textDimmed}`,
          }} />
        )}
      </div>
    </div>
  )
}
