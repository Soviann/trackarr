import { useState } from 'preact/hooks'
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
  onAniList?: () => void
}

export function ActionDrawer({
  title, nextEpisode, nextSeasonNumber,
  onMarkNext, onRate, onEdit, onRematch, onAniList,
}: ActionDrawerProps) {
  const [open, setOpen] = useState(false)

  const hasImdb = !!title.imdb_id
  const hasAnilist = title.type === 'anime'
  const hasSeries = title.type !== 'movie'

  return (
    <div className={s.container}>
      <div className={s.handle} onClick={() => setOpen(!open)}>
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
        </div>

        <div className={s.bottomPad} />
      </div>
    </div>
  )
}
