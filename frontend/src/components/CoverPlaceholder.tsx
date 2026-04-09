import type { JSX } from 'preact'
import type { TitleType } from '../types'
import { colors } from '../theme'
import s from './CoverPlaceholder.module.css'

const typeConfig: Record<TitleType | 'anime' | 'unknown', { color: string; icon: JSX.Element }> = {
  movie: {
    color: colors.accentBlue,
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18" />
        <line x1="7" y1="2" x2="7" y2="22" />
        <line x1="17" y1="2" x2="17" y2="22" />
        <line x1="12" y1="2" x2="12" y2="22" />
        <line x1="2" y1="12" x2="22" y2="12" />
        <line x1="2" y1="7" x2="7" y2="7" />
        <line x1="2" y1="17" x2="7" y2="17" />
        <line x1="17" y1="7" x2="22" y2="7" />
        <line x1="17" y1="17" x2="22" y2="17" />
      </svg>
    ),
  },
  series: {
    color: colors.accentTeal,
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <rect x="2" y="7" width="20" height="15" rx="2" ry="2" />
        <polyline points="17 2 12 7 7 2" />
      </svg>
    ),
  },
  anime: {
    color: colors.accentLavender,
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
      </svg>
    ),
  },
  unknown: {
    color: colors.textMuted,
    icon: (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10" />
        <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
        <line x1="12" y1="17" x2="12.01" y2="17" />
      </svg>
    ),
  },
}

interface CoverPlaceholderProps {
  type: TitleType
  is_anime?: boolean
  /** Icon size in px (default: 40% of container) */
  iconSize?: string
}

export function CoverPlaceholder({ type, is_anime, iconSize }: CoverPlaceholderProps) {
  const config = typeConfig[is_anime ? 'anime' : type] || typeConfig.unknown
  const { color, icon } = config
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
export function coverBackground(coverUrl: string | null, type: TitleType, is_anime?: boolean): string {
  if (coverUrl) return `url(/api/covers/${coverUrl}) center/cover`
  const config = typeConfig[is_anime ? 'anime' : type] || typeConfig.unknown
  const { color } = config
  return `linear-gradient(135deg, ${color}25, ${color}0A)`
}
