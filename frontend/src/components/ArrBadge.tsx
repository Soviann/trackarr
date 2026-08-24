import clsx from 'clsx'
import s from './ArrBadge.module.css'

interface ArrBadgeProps {
  type: 'movie' | 'series'
  radarrId?: number | null
  sonarrId?: number | null
}

export function ArrBadge({ type, radarrId, sonarrId }: ArrBadgeProps) {
  const isRadarr = type === 'movie' && radarrId != null
  const isSonarr = type === 'series' && sonarrId != null

  if (!isRadarr && !isSonarr) return null

  if (isRadarr) {
    return (
      <span
        className={`${s.badge} ${s.radarr}`}
        aria-label="Tracked on Radarr"
        title="Tracked on Radarr"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <circle cx="12" cy="12" r="10" />
          <line x1="12" y1="2" x2="12" y2="22" />
          <line x1="2" y1="12" x2="22" y2="12" />
        </svg>
      </span>
    )
  }

  return (
    <span
      className={`${s.badge} ${s.sonarr}`}
      aria-label="Tracked on Sonarr"
      title="Tracked on Sonarr"
    >
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
        <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
      </svg>
    </span>
  )
}
