import type { Title, Episode } from '../types'
import { colors, accentWash } from '../theme'

interface ActionBarProps {
  title: Title
  nextEpisode: Episode | null
  nextSeasonNumber?: number
  onMarkNext?: () => void
  onRate?: () => void
  onAniList?: () => void
}

export function ActionBar({ title, nextEpisode, nextSeasonNumber, onMarkNext, onRate, onAniList }: ActionBarProps) {
  const hasImdb = !!title.imdb_id
  const hasAnilist = title.type === 'anime'

  return (
    <div style={{
      display: 'flex',
      borderTop: `1px solid ${colors.borderSubtle}`,
      background: colors.bgPrimary,
      position: 'fixed',
      bottom: '72px',
      left: 0,
      right: 0,
      zIndex: 99,
    }}>
      {/* Next unwatched */}
      {nextEpisode && (
        <button
          onClick={onMarkNext}
          style={{
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '5px',
            padding: '8px 0 10px',
            background: accentWash(colors.accentCoral),
            borderTop: `2px solid ${colors.accentCoral}`,
            border: 'none',
            borderTopStyle: 'solid',
            borderTopWidth: '2px',
            borderTopColor: colors.accentCoral,
            cursor: 'pointer',
          }}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.accentCoral} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="20 6 9 17 4 12" />
          </svg>
          <span style={{ fontSize: '9px', fontWeight: 600, color: colors.accentCoral }}>
            S{String(nextSeasonNumber ?? 1).padStart(2, '0')}E{String(nextEpisode.episode).padStart(2, '0')}
          </span>
        </button>
      )}

      {/* IMDb link */}
      {hasImdb && (
        <a
          href={`https://www.imdb.com/title/${title.imdb_id}/`}
          target="_blank"
          rel="noopener noreferrer"
          style={{
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '5px',
            padding: '8px 0 10px',
            borderTop: '2px solid transparent',
            textDecoration: 'none',
          }}
        >
          <span style={{ fontSize: '12px', fontWeight: 800, color: colors.accentImdb, fontFamily: 'Impact,system-ui' }}>
            IMDb
          </span>
        </a>
      )}

      {/* AniList */}
      {hasAnilist && (
        <button
          onClick={onAniList}
          style={{
            flex: 1,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: '5px',
            padding: '8px 0 10px',
            borderTop: '2px solid transparent',
            border: 'none',
            background: 'transparent',
            cursor: 'pointer',
          }}
        >
          <span style={{ fontSize: '11px', fontWeight: 700, color: colors.accentAnilist }}>AniList</span>
        </button>
      )}

      {/* Rate */}
      <button
        onClick={onRate}
        style={{
          flex: 1,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          gap: '5px',
          padding: '8px 0 10px',
          borderTop: '2px solid transparent',
          border: 'none',
          background: 'transparent',
          cursor: 'pointer',
        }}
      >
        <svg width="16" height="16" viewBox="0 0 24 24"
          stroke={title.my_rating ? colors.accentAmber : colors.textMuted}
          stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polygon style={{ fill: title.my_rating ? colors.accentAmber : 'none' }} points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
        </svg>
        {title.my_rating ? (
          <span style={{ fontSize: '9px', fontWeight: 500, color: colors.accentAmber }}>
            {title.my_rating}/10
          </span>
        ) : (
          <span style={{ fontSize: '9px', color: colors.textMuted }}>Rate</span>
        )}
      </button>
    </div>
  )
}
