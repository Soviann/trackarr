import { memo } from 'preact/compat'
import { useRef, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import clsx from 'clsx'
import type { Title } from '../types'
import { apiFetch } from '../api'
import { getName, formatSortCaption } from '../utils'
import { useTranslation } from '../i18n'
import { useTitleStore } from '../store'
import { routeTo } from '../routes'
import { CoverImage } from './CoverImage'
import { StatusBadge } from './StatusBadge'
import { TypeBadge } from './TypeBadge'
import { WatchProviderBadges } from './WatchProviderBadges'
import { useLongPress } from '../hooks/useLongPress'
import s from './PosterCard.module.css'

interface PosterCardProps {
  title: Title
  onClick?: (e: MouseEvent) => void
  onLongPress?: () => void
  onUpdate?: () => void
  overlay?: ComponentChildren
  selecting?: boolean
}

export const PosterCard = memo(function PosterCard({ title, onClick, onLongPress, onUpdate, overlay, selecting }: PosterCardProps) {
  const { t } = useTranslation()
  const sortField = useTitleStore(st => st.sort.field)
  const sortCaption = formatSortCaption(title, sortField)
  const name = getName(title)
  const [toggling, setToggling] = useState(false)
  const ne = title.next_episode

  const handleQuickMark = async (e: MouseEvent) => {
    e.stopPropagation()
    e.preventDefault()
    if (!ne || toggling) return
    setToggling(true)
    try {
      await apiFetch(`/titles/${title.id}/episodes/${ne.id}`, { method: 'PATCH' })
      onUpdate?.()
    } catch (err) {
      console.error('Failed to mark episode:', err)
    } finally {
      setToggling(false)
    }
  }

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
  const hasArr = title.sonarr_id != null || title.radarr_id != null
  const isTBA = Boolean(ne?.is_tba || (ne?.name && (ne.name.trim().toUpperCase() === 'TBA' || ne.name.trim().toUpperCase() === 'TBD')))

  return (
    <a
      href={routeTo.title(title.id)}
      className={cardClass}
      {...longPressHandlers}
      onPointerDown={handlePointerDown}
      onClick={handleClick}
    >
      <div className={s.poster}>
        <CoverImage
          coverUrl={title.cover_url}
          type={title.type}
          is_anime={title.is_anime}
          alt={name}
          className={s.coverImage}
        />
        <div className={`${s.badges}${selecting ? ` ${s.badgesShifted}` : ''}`}>
          <TypeBadge type={title.type} radarrId={title.radarr_id} sonarrId={title.sonarr_id} />
          <WatchProviderBadges providers={title.watch_providers} />
        </div>

        {/* Arr availability badge (top-right) */}
        {hasArr && ne && !selecting && title.status !== 'dropped' && !isTBA && (
          <span className={s.arrAvailableBadge}>
            {`S${ne.season_number.toString().padStart(2, '0')}E${ne.episode.toString().padStart(2, '0')} ${t('common.dispoBadge')}`}
          </span>
        )}

        <div className={s.statusBadge}>
          <StatusBadge status={title.status} caughtUp={title.caught_up} />
        </div>

        {/* Symmetrical +1 Action Button: equal bottom & right offset (10px) */}
        {title.status === 'watching' && ne && !selecting && (
          <button
            type="button"
            className={clsx(s.quickPlusBtn, toggling && s.quickPlusBtnLoading)}
            onClick={handleQuickMark}
            disabled={toggling}
            aria-label={`Mark S${ne.season_number} E${ne.episode} as watched`}
            title={t('common.markNextWatched')}
          >
            {toggling ? <span className={s.quickMarkSpinner} aria-hidden="true" /> : '+1'}
          </button>
        )}

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
