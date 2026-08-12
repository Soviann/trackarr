import type { TitleType } from '../types'
import { getCoverUrl } from '../utils'
import { typeIconConfig, resolveTypeIconKey } from './typeIcons'
import s from './CoverPlaceholder.module.css'

interface CoverPlaceholderProps {
  type: TitleType
  is_anime?: boolean
  /** Icon size in px (default: 40% of container) */
  iconSize?: string
}

export function CoverPlaceholder({ type, is_anime, iconSize }: CoverPlaceholderProps) {
  const { color, icon } = typeIconConfig[resolveTypeIconKey(type, is_anime)]
  return (
    <div
      className={s.placeholder}
      style={{
        '--cover-color': color,
        ...(iconSize ? { '--icon-size': iconSize } : {}),
      } as Record<string, string>}
    >
      <div className={s.icon}>
        {icon}
      </div>
    </div>
  )
}

/** CSS background string for cover or placeholder gradient */
export function coverBackground(coverUrl: string | null | undefined, type: TitleType, is_anime?: boolean): string {
  const url = getCoverUrl(coverUrl)
  if (url) return `url(${url}) center/cover`
  const { color } = typeIconConfig[resolveTypeIconKey(type, is_anime)]
  return `linear-gradient(135deg, ${color}25, ${color}0A)`
}
