import clsx from 'clsx'
import type { TitleType } from '../types'
import { typeIconConfig } from './typeIcons'
import { useTranslation } from '../i18n'
import s from './TypeBadge.module.css'

type TypeBadgeSize = 'sm' | 'md'

interface TypeBadgeProps {
  type: TitleType
  size?: TypeBadgeSize
  radarrId?: number | null
  sonarrId?: number | null
}

export function TypeBadge({ type, size = 'md', radarrId, sonarrId }: TypeBadgeProps) {
  const { t } = useTranslation()
  const { color, icon } = typeIconConfig[type]
  const hasRadarr = type === 'movie' && radarrId != null
  const hasSonarr = type === 'series' && sonarrId != null

  const arrTitle = hasRadarr
    ? t('arrBadge.trackedOnRadarr')
    : hasSonarr
    ? t('arrBadge.trackedOnSonarr')
    : undefined
  const arrLabel = arrTitle ? ` (${arrTitle})` : ''

  return (
    <div
      role="img"
      className={clsx(
        s.badge,
        size === 'sm' ? s.sizeSm : s.sizeMd,
        hasRadarr && s.hasRadarr,
        hasSonarr && s.hasSonarr
      )}
      style={{ color }}
      aria-label={(type === 'movie' ? t('common.movie') : t('common.series')) + arrLabel}
      title={arrTitle}
    >
      <div className={clsx(s.icon, size === 'sm' ? s.iconSm : s.iconMd)}>
        {icon}
      </div>
    </div>
  )
}

