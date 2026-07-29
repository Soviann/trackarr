import { memo } from 'preact/compat'
import { useRef, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import type { Title } from '../types'
import { getName, formatSortCaption } from '../utils'
import { useTitleStore } from '../store'
import { routeTo } from '../routes'
import { CoverPlaceholder } from './CoverPlaceholder'
import { StatusBadge } from './StatusBadge'
import { TypeBadge } from './TypeBadge'
import { useLongPress } from '../hooks/useLongPress'
import s from './PosterCard.module.css'

interface PosterCardProps {
  title: Title
  onClick?: (e: MouseEvent) => void
  onLongPress?: () => void
  overlay?: ComponentChildren
  selecting?: boolean
}

export const PosterCard = memo(function PosterCard({ title, onClick, onLongPress, overlay, selecting }: PosterCardProps) {
  const sortField = useTitleStore(st => st.sort.field)
  const sortCaption = formatSortCaption(title, sortField)
  const name = getName(title)
  const [imgError, setImgError] = useState(false)

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
      href={routeTo.title(title.id)}
      className={cardClass}
      {...longPressHandlers}
      onPointerDown={handlePointerDown}
      onClick={handleClick}
    >
      <div className={s.poster}>
        {title.cover_url && !imgError ? (
          <img
            src={`/api/covers/${title.cover_url}`}
            className={s.coverImage}
            loading="lazy"
            decoding="async"
            alt={name}
            onError={() => setImgError(true)}
          />
        ) : (
          <CoverPlaceholder type={title.type} />
        )}
        <div className={`${s.typeBadge}${selecting ? ` ${s.typeBadgeShifted}` : ''}`}>
          <TypeBadge type={title.type} />
        </div>
        <div className={s.statusBadge}>
          <StatusBadge status={title.status} caughtUp={title.caught_up} />
        </div>
        <div className={s.labelOverlay}>
          <div className={s.label}>{name}</div>
          {sortCaption && (
            <div className={s.sortCaption}>{sortCaption}</div>
          )}
        </div>
        {overlay}
      </div>
    </a>
  )
})
