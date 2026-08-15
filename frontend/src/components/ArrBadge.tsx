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
        Radarr
      </span>
    )
  }

  return (
    <span
      className={clsx(s.badge, s.sonarr)}
      aria-label="Présent sur Sonarr"
      title="Présent sur Sonarr"
    >
      Sonarr
    </span>
  )
}
