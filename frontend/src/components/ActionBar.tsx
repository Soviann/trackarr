import clsx from 'clsx'
import type { Title, Episode } from '../types'
import { colors } from '../theme'
import s from './ActionBar.module.css'

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
    <div className={s.bar}>
      {/* Next unwatched */}
      {nextEpisode && (
        <button
          onClick={onMarkNext}
          className={clsx(s.action, s.markNext)}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke={colors.accentCoral} stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="20 6 9 17 4 12" />
          </svg>
          <span className={s.markNextLabel}>
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
          className={s.action}
        >
          <span className={s.imdbLabel}>IMDb</span>
        </a>
      )}

      {/* AniList */}
      {hasAnilist && (
        <button onClick={onAniList} className={s.action}>
          <span className={s.anilistLabel}>AniList</span>
        </button>
      )}

      {/* Rate */}
      <button onClick={onRate} className={s.action}>
        <svg width="16" height="16" viewBox="0 0 24 24"
          stroke={title.my_rating ? colors.accentAmber : colors.textMuted}
          stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polygon style={{ fill: title.my_rating ? colors.accentAmber : 'none' }} points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
        </svg>
        {title.my_rating ? (
          <span className={s.ratingLabel}>{title.my_rating}/10</span>
        ) : (
          <span className={s.ratePlaceholder}>Rate</span>
        )}
      </button>
    </div>
  )
}
