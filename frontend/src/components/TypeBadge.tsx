import clsx from 'clsx'
import type { TitleType } from '../types'
import { typeIconConfig } from './typeIcons'
import s from './TypeBadge.module.css'

type TypeBadgeSize = 'sm' | 'md'

interface TypeBadgeProps {
  type: TitleType
  size?: TypeBadgeSize
  radarrId?: number | null
  sonarrId?: number | null
}

export function TypeBadge({ type, size = 'md', radarrId, sonarrId }: TypeBadgeProps) {
  const { color, icon } = typeIconConfig[type]
  const hasRadarr = type === 'movie' && radarrId != null
  const hasSonarr = type === 'series' && sonarrId != null

  const arrLabel = hasRadarr
    ? ' (Tracked on Radarr)'
    : hasSonarr
    ? ' (Tracked on Sonarr)'
    : ''

  return (
    <div
      className={clsx(
        s.badge,
        size === 'sm' ? s.sizeSm : s.sizeMd,
        hasRadarr && s.hasRadarr,
        hasSonarr && s.hasSonarr
      )}
      style={{ color }}
      aria-label={(type === 'movie' ? 'Movie' : 'Series') + arrLabel}
      title={hasRadarr ? 'Tracked on Radarr' : hasSonarr ? 'Tracked on Sonarr' : undefined}
    >
      <div className={clsx(s.icon, size === 'sm' ? s.iconSm : s.iconMd)}>
        {icon}
      </div>
    </div>
  )
}

