import { memo } from 'preact/compat'
import type { ComponentChildren } from 'preact'
import type { Title } from '../types'
import { getName, formatDate } from '../utils'
import { useTitleStore } from '../store'
import { CoverPlaceholder, coverBackground } from './CoverPlaceholder'
import { StatusBadge } from './StatusBadge'
import s from './PosterCard.module.css'

interface PosterCardProps {
  title: Title
  onClick?: (e: MouseEvent) => void
  overlay?: ComponentChildren
}

export const PosterCard = memo(function PosterCard({ title, onClick, overlay }: PosterCardProps) {
  const isLastWatchedSort = useTitleStore(s => s.sort.field === 'last_watched_at')
  const name = getName(title)

  const handleClick = (e: MouseEvent) => {
    if (!onClick) return
    e.preventDefault()
    e.stopPropagation()
    onClick(e)
  }

  return (
    <a href={`/title/${title.id}`} onClick={handleClick} className={s.card}>
      <div
        className={s.poster}
        style={{ background: coverBackground(title.cover_url, title.type) }}
      >
        {!title.cover_url && <CoverPlaceholder type={title.type} />}
        <div className={s.statusBadge}>
          <StatusBadge status={title.status} />
        </div>
        <div className={s.labelOverlay}>
          <div className={s.label}>{name}</div>
          {isLastWatchedSort && title.last_watched_at && (
            <div className={s.lastWatched}>Vu le {formatDate(title.last_watched_at)}</div>
          )}
        </div>
        {overlay}
      </div>
    </a>
  )
})
