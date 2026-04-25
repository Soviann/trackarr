import { memo } from 'preact/compat'
import { useRef } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import type { Title } from '../types'
import { getName, formatDate } from '../utils'
import { useTitleStore } from '../store'
import { CoverPlaceholder, coverBackground } from './CoverPlaceholder'
import { StatusBadge } from './StatusBadge'
import { TypeBadge } from './TypeBadge'
import { useLongPress } from '../hooks/useLongPress'
import s from './PosterCard.module.css'

interface PosterCardProps {
  title: Title
  onClick?: (e: MouseEvent) => void
  onLongPress?: () => void
  overlay?: ComponentChildren
}

export const PosterCard = memo(function PosterCard({ title, onClick, onLongPress, overlay }: PosterCardProps) {
  const isLastWatchedSort = useTitleStore(st => st.sort.field === 'last_watched_at')
  const name = getName(title)

  // Track whether a long-press just fired so we can swallow the
  // synthetic click that the browser dispatches after pointer-up.
  const justFiredRef = useRef(false)

  const longPressHandlers = useLongPress({
    onLongPress: () => {
      justFiredRef.current = true
      onLongPress?.()
    },
  })

  const handlePointerDown = (e: PointerEvent) => {
    justFiredRef.current = false
    longPressHandlers.onPointerDown(e)
  }

  const handleClick = (e: MouseEvent) => {
    if (justFiredRef.current) {
      e.preventDefault()
      e.stopPropagation()
      justFiredRef.current = false
      return
    }
    if (!onClick) return
    e.preventDefault()
    e.stopPropagation()
    onClick(e)
  }

  // Apply no-touch-callout when a long-press handler is provided so the
  // user doesn't see the "Save image" native callout during the hold.
  const cardClass = `${s.card}${onLongPress ? ' no-touch-callout' : ''}`

  return (
    <a
      href={`/title/${title.id}`}
      className={cardClass}
      {...longPressHandlers}
      onPointerDown={handlePointerDown}
      onClick={handleClick}
    >
      <div
        className={s.poster}
        style={{ background: coverBackground(title.cover_url, title.type) }}
      >
        {!title.cover_url && <CoverPlaceholder type={title.type} />}
        <div className={s.typeBadge}>
          <TypeBadge type={title.type} />
        </div>
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
