import { createPortal } from 'preact/compat'
import { useEffect, useMemo } from 'preact/hooks'
import type { CSSProperties } from 'preact'
import { route } from 'preact-router'
import type { Title } from '../types'
import { getName, isUnairedOrTBA } from '../utils'
import { useTranslation } from '../i18n'
import { routeTo } from '../routes'
import { CoverImage } from './CoverImage'
import { StatusBadge } from './StatusBadge'
import { TypeBadge } from './TypeBadge'
import { WatchProviderBadges } from './WatchProviderBadges'
import s from './PosterHoverCard.module.css'

export interface PosterHoverCardProps {
  title: Title
  anchorRect: DOMRect
  onClose: () => void
  onPointerEnter?: () => void
  onPointerLeave?: () => void
  onQuickMark?: (e: MouseEvent) => void
  quickMarkLoading?: boolean
}

export function PosterHoverCard({
  title,
  anchorRect,
  onClose,
  onPointerEnter,
  onPointerLeave,
  onQuickMark,
  quickMarkLoading = false,
}: PosterHoverCardProps) {
  const { t } = useTranslation()
  const name = getName(title)
  const ne = title.next_episode
  const isUnaired = isUnairedOrTBA(ne)
  const hasQuickAction = title.status === 'watching' && ne != null && !isUnaired

  // Calculate clamped fixed positioning centered over anchor card
  const style = useMemo(() => {
    const cardWidth = 300
    const estimatedHeight = 260

    let left = anchorRect.left + (anchorRect.width - cardWidth) / 2
    if (typeof window !== 'undefined') {
      left = Math.max(12, Math.min(window.innerWidth - cardWidth - 12, left))
    }

    let top = anchorRect.top - 8
    if (typeof window !== 'undefined') {
      if (top + estimatedHeight > window.innerHeight - 12) {
        top = Math.max(12, window.innerHeight - estimatedHeight - 12)
      } else if (top < 12) {
        top = 12
      }
    }

    const accent = title.accent_hex || 'var(--accent)'

    return {
      left: `${left}px`,
      top: `${top}px`,
      '--hover-accent': accent,
    }
  }, [anchorRect, title.accent_hex])

  // Close on window scroll or escape key
  useEffect(() => {
    const handleScroll = () => onClose()
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }

    window.addEventListener('scroll', handleScroll, { passive: true, capture: true })
    window.addEventListener('keydown', handleKeyDown)

    return () => {
      window.removeEventListener('scroll', handleScroll, { capture: true })
      window.removeEventListener('keydown', handleKeyDown)
    }
  }, [onClose])

  const handleClick = (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    onClose()
    route(routeTo.title(title.id))
  }

  const handleQuickMarkClick = (e: MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    if (!quickMarkLoading && onQuickMark) {
      onQuickMark(e)
    }
  }

  if (typeof document === 'undefined') return null

  return createPortal(
    <div
      className={s.portal}
      style={style as CSSProperties}
      onPointerEnter={onPointerEnter}
      onPointerLeave={onPointerLeave}
      onClick={handleClick}
      aria-label={name}
    >
      <div className={s.header}>
        <div className={s.thumbWrap}>
          <CoverImage
            coverUrl={title.cover_url}
            type={title.type}
            is_anime={title.is_anime}
            alt={name}
            className={s.thumb}
          />
        </div>
        <div className={s.headerInfo}>
          <div className={s.titleText}>{name}</div>
          <div className={s.metaRow}>
            {title.year > 0 && <span>{title.year}</span>}
            {title.runtime != null && title.runtime > 0 && <span>· {title.runtime}m</span>}
          </div>
          <div className={s.badgesRow}>
            <TypeBadge type={title.type} radarrId={title.radarr_id} sonarrId={title.sonarr_id} />
            <StatusBadge status={title.status} caughtUp={title.caught_up} />
          </div>
        </div>
      </div>

      <div className={s.body}>
        {/* Ratings row */}
        {(title.tmdb_rating != null || title.anilist_rating != null || title.my_rating != null) && (
          <div className={s.ratingsRow}>
            {title.tmdb_rating != null && (
              <span className={s.ratingItem}>
                <span className={s.ratingTmdb}>★ {title.tmdb_rating.toFixed(1)}</span> TMDB
              </span>
            )}
            {title.anilist_rating != null && (
              <span className={s.ratingItem}>
                <span className={s.ratingAnilist}>{title.anilist_rating}%</span> AniList
              </span>
            )}
            {title.my_rating != null && (
              <span className={s.ratingItem}>
                <span className={s.ratingMy}>★ {title.my_rating}</span> /10
              </span>
            )}
          </div>
        )}

        {/* Synopsis */}
        {title.overview ? (
          <p className={s.synopsis}>{title.overview}</p>
        ) : (
          <p className={s.synopsisEmpty}>{t('details.overview')}: —</p>
        )}

        {/* Next episode row with quick mark */}
        {hasQuickAction && ne && (
          <div className={s.nextEpisodeCard}>
            <div className={s.nextEpisodeInfo}>
              <span className={s.nextEpisodeLabel}>{t('common.dispoBadge')}</span>
              <span className={s.nextEpisodeCode}>
                {`S${ne.season_number}E${ne.episode}${ne.name ? ` — ${ne.name}` : ''}`}
              </span>
            </div>
            <button
              type="button"
              className={s.quickPlusBtn}
              onClick={handleQuickMarkClick}
              disabled={quickMarkLoading}
              title={t('common.markNextWatched')}
              aria-label={t('common.markNextWatched')}
            >
              {quickMarkLoading ? (
                <span className={s.quickMarkSpinner} aria-hidden="true" />
              ) : (
                <>+1</>
              )}
            </button>
          </div>
        )}

        {/* Watch Providers */}
        {title.watch_providers && title.watch_providers.length > 0 && (
          <div className={s.providersWrap} onClick={(e) => e.stopPropagation()}>
            <WatchProviderBadges providers={title.watch_providers} />
          </div>
        )}
      </div>
    </div>,
    document.body
  )
}
