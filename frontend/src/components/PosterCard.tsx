import { memo } from 'preact/compat'
import { useRef, useState } from 'preact/hooks'
import type { ComponentChildren } from 'preact'
import clsx from 'clsx'
import type { Title } from '../types'
import { apiFetch } from '../api'
import { getName, formatSortCaption, isUnairedOrTBA } from '../utils'
import { useTranslation } from '../i18n'
import { useTitleStore } from '../store'
import { routeTo } from '../routes'
import { CoverImage } from './CoverImage'
import { StatusBadge } from './StatusBadge'
import { TypeBadge } from './TypeBadge'
import { useLongPress } from '../hooks/useLongPress'
import { haptic } from '../utils/haptic'
import { useUndo } from '../context/UndoContext'
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
  const { showUndo } = useUndo()
  const sortField = useTitleStore(st => st.sort.field)
  const sortCaption = formatSortCaption(title, sortField)
  const name = getName(title)
  const [toggling, setToggling] = useState(false)
  const [popping, setPopping] = useState(false)
  const ne = title.next_episode

  const handleQuickMark = async (e: MouseEvent) => {
    e.stopPropagation()
    e.preventDefault()
    if (!ne || toggling) return
    haptic([15, 30, 15])
    setPopping(true)
    setTimeout(() => setPopping(false), 450)
    setToggling(true)
    const targetEpisode = ne
    try {
      await apiFetch(`/titles/${title.id}/episodes/${targetEpisode.id}`, { method: 'PATCH' })
      onUpdate?.()
      showUndo({
        message: t('undo.quickMarked', {
          title: name,
          ep: `S${targetEpisode.season_number}E${targetEpisode.episode}`,
        }),
        onUndo: async () => {
          await apiFetch(`/titles/${title.id}/episodes/${targetEpisode.id}`, { method: 'PATCH' })
          onUpdate?.()
        },
      })
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
  const isUnaired = isUnairedOrTBA(ne)
  const hasQuickAction = title.status === 'watching' && ne != null && !selecting && !isUnaired

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
        </div>

        {/* Arr availability badge (top-right) */}
        {hasArr && ne && !selecting && title.status !== 'dropped' && !isUnaired && (
          <span className={s.arrAvailableBadge}>
            {`S${ne.season_number.toString().padStart(2, '0')}E${ne.episode.toString().padStart(2, '0')} ${t('common.dispoBadge')}`}
          </span>
        )}

        <div className={s.statusBadge}>
          <StatusBadge status={title.status} caughtUp={title.caught_up} />
        </div>

        {/* Quick +1 Action Button: equal bottom & right offset (8px) */}
        {hasQuickAction && (
          <button
            type="button"
            className={clsx(s.quickPlusBtn, toggling && s.quickPlusBtnLoading, popping && s.quickPlusBtnPopping)}
            onClick={handleQuickMark}
            disabled={toggling}
            aria-label={`Mark S${ne.season_number} E${ne.episode} as watched`}
            title={t('common.markNextWatched')}
          >
            {toggling ? (
              <span className={s.quickMarkSpinner} aria-hidden="true" />
            ) : (
              <>
                <span className={s.quickPlusSign}>+</span>
                <span className={s.quickPlusNum}>1</span>
              </>
            )}
          </button>
        )}

        <div className={clsx(s.labelOverlay, hasQuickAction && s.labelOverlayWithAction)}>
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
