import type { Season } from '../types'
import { colors } from '../theme'

interface SeasonTabProps {
  season: Season
  active: boolean
  onClick: () => void
}

export function SeasonTab({ season, active, onClick }: SeasonTabProps) {
  const eps = season.episodes ?? []
  const watched = eps.filter((e) => e.watched).length
  const total = season.total_episodes ?? eps.length
  const allWatched = total > 0 && watched >= total

  if (active) {
    return (
      <button onClick={onClick} style={{
        background: colors.accentAmber,
        borderRadius: '14px',
        padding: '5px 14px 5px 12px',
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        border: 'none',
        cursor: 'pointer',
      }}>
        <span style={{ fontSize: '11px', fontWeight: 600, color: colors.bgPrimary }}>
          S{season.season_number}
        </span>
        <span style={{ fontSize: '9px', color: 'rgba(13,13,13,0.6)' }}>
          {watched}/{total}
        </span>
      </button>
    )
  }

  return (
    <button onClick={onClick} style={{
      background: colors.bgSurface,
      borderRadius: '14px',
      padding: '5px 14px 5px 12px',
      display: 'flex',
      alignItems: 'center',
      gap: '6px',
      border: 'none',
      cursor: 'pointer',
    }}>
      {allWatched && (
        <svg width="12" height="12" viewBox="0 0 24 24" fill={colors.accentGreen} stroke="none">
          <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" />
        </svg>
      )}
      <span style={{ fontSize: '11px', color: '#aaa' }}>S{season.season_number}</span>
      {season.my_rating != null && (
        <span style={{ fontSize: '9px', color: colors.textMuted }}>
          ★ {season.my_rating}
        </span>
      )}
    </button>
  )
}
