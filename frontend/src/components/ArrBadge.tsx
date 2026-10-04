import { useTranslation } from '../i18n'
import s from './ArrBadge.module.css'

interface ArrBadgeProps {
  type: 'movie' | 'series'
  radarrId?: number | null
  sonarrId?: number | null
}

export function ArrBadge({ type, radarrId, sonarrId }: ArrBadgeProps) {
  const { t } = useTranslation()
  const isRadarr = type === 'movie' && radarrId != null
  const isSonarr = type === 'series' && sonarrId != null

  if (!isRadarr && !isSonarr) return null

  if (isRadarr) {
    const label = t('arrBadge.trackedOnRadarr')
    return (
      <span
        className={`${s.badge} ${s.radarr}`}
        aria-label={label}
        title={label}
      >
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <circle cx="12" cy="12" r="10" />
          <line x1="12" y1="2" x2="12" y2="22" />
          <line x1="2" y1="12" x2="22" y2="12" />
        </svg>
      </span>
    )
  }

  const label = t('arrBadge.trackedOnSonarr')
  return (
    <span
      className={`${s.badge} ${s.sonarr}`}
      aria-label={label}
      title={label}
    >
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
        <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
      </svg>
    </span>
  )
}
