import { useState, useRef } from 'preact/hooks'
import clsx from 'clsx'
import type { Title, Episode } from '../types'
import s from './ActionDrawer.module.css'

interface ActionDrawerProps {
  title: Title
  nextEpisode: Episode | null
  nextSeasonNumber?: number
  onMarkNext?: () => void
  onRate: () => void
  onEdit: () => void
  onRematch: () => void
  onMerge: () => void
  onAniList?: () => void
}

export function ActionDrawer({
  title, nextEpisode, nextSeasonNumber,
  onMarkNext, onRate, onEdit, onRematch, onMerge, onAniList,
}: ActionDrawerProps) {
  const [open, setOpen] = useState(false)
  const [dragY, setDragY] = useState(0)
  const touchStartY = useRef<number | null>(null)

  const handleTouchStart = (e: TouchEvent) => {
    if (!open) return
    touchStartY.current = e.touches[0].clientY
  }

  const handleTouchMove = (e: TouchEvent) => {
    if (!open || touchStartY.current === null) return
    const deltaY = e.touches[0].clientY - touchStartY.current
    if (deltaY > 0) {
      setDragY(deltaY)
    }
  }

  const handleTouchEnd = () => {
    if (!open || touchStartY.current === null) return
    if (dragY > 100) {
      setOpen(false)
    }
    setDragY(0)
    touchStartY.current = null
  }

  const hasImdb = !!title.imdb_id
  const hasAnilist = title.is_anime
  const hasSeries = title.type !== 'movie'

  return (
    <div
      className={s.container}
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
      style={dragY > 0 ? { transform: `translateY(${dragY}px)`, transition: 'none' } : undefined}
    >
      <div
        className={s.handle}
        onClick={() => setOpen(!open)}
      >
        <div className={s.handleBar} />
        <span className={s.handleText}>Actions</span>
        <span className={clsx(s.chevron, open && s.chevronOpen)}>&#9650;</span>
      </div>

      <div className={clsx(s.drawer, open ? s.drawerExpanded : s.drawerCollapsed)}>
        <div className={clsx(s.sectionLabel, s.sectionLabelFirst)}>Quick actions</div>
        <div className={s.actionRow}>
          {hasSeries && nextEpisode && (
            <button onClick={onMarkNext} className={s.markNext}>
              ✓ S{String(nextSeasonNumber ?? 1).padStart(2, '0')}E{String(nextEpisode.episode).padStart(2, '0')}
            </button>
          )}
          <button onClick={onRate} className={s.rate}>
            ★ Rate
          </button>
          {hasImdb && (
            <a
              href={`https://www.imdb.com/title/${title.imdb_id}/`}
              target="_blank"
              rel="noopener noreferrer"
              className={s.imdb}
            >
              IMDb
            </a>
          )}
          {hasAnilist && (
            <button onClick={onAniList} className={s.anilist}>
              AniList
            </button>
          )}
        </div>

        <div className={s.sectionLabel}>Manage</div>
        <div className={s.actionRow}>
          <button onClick={onEdit} className={s.manage}>
            ✎ Edit
          </button>
          <button onClick={onRematch} className={s.manage}>
            🔍 Fix match
          </button>
          <button onClick={onMerge} className={s.manage}>
            🔗 Merge into...
          </button>
        </div>

        <div className={s.bottomPad} />
      </div>
    </div>
  )
}
