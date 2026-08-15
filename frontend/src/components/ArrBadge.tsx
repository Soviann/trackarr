import clsx from 'clsx'
import s from './ArrBadge.module.css'

interface ArrBadgeProps {
  type: 'movie' | 'series'
  radarrId?: number | null
  sonarrId?: number | null
}

export function ArrBadge({ type, radarrId, sonarrId }: ArrBadgeProps) {
  const hasRadarr = type === 'movie' && radarrId != null
  const hasSonarr = type === 'series' && sonarrId != null

  if (!hasRadarr && !hasSonarr) return null

  if (hasRadarr) {
    return (
      <span
        className={clsx(s.badge, s.radarr)}
        aria-label="Présent sur Radarr"
        title="Présent sur Radarr"
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10" />
          <polygon points="16.24 7.76 14.12 14.12 7.76 16.24 9.88 9.88 16.24 7.76" />
        </svg>
      </span>
    )
  }

  return (
    <span
      className={clsx(s.badge, s.sonarr)}
      aria-label="Présent sur Sonarr"
      title="Présent sur Sonarr"
    >
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M2 12h4l3-9 5 18 3-9h5" />
      </svg>
    </span>
  )
}
