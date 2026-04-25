import clsx from 'clsx'
import type { TitleType } from '../types'
import { typeIconConfig } from './typeIcons'
import s from './TypeBadge.module.css'

type TypeBadgeSize = 'sm' | 'md'

interface TypeBadgeProps {
  type: TitleType
  size?: TypeBadgeSize
}

export function TypeBadge({ type, size = 'md' }: TypeBadgeProps) {
  const { color, icon } = typeIconConfig[type]
  return (
    <div
      className={clsx(s.badge, size === 'sm' ? s.sizeSm : s.sizeMd)}
      style={{ color }}
      aria-label={type === 'movie' ? 'Movie' : 'Series'}
    >
      <div className={clsx(s.icon, size === 'sm' ? s.iconSm : s.iconMd)}>
        {icon}
      </div>
    </div>
  )
}
