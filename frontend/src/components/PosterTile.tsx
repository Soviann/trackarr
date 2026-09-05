import clsx from 'clsx'
import { route } from 'preact-router'
import type { TitleType, WatchProvider, NextEpisode } from '../types'
import { useTranslation } from '../i18n'
import s from './PosterTile.module.css'
import { CoverImage } from './CoverImage'
import { WatchProviderBadges } from './WatchProviderBadges'
import { TypeBadge } from './TypeBadge'

export interface PosterTileItem {
  id: number
  type: TitleType
  is_anime?: boolean
  sonarr_id?: number | null
  radarr_id?: number | null
  cover_url: string | null
  name: string
  sublabel: string
  sublabelVariant?: 'default' | 'amber' | 'teal' | 'muted'
  progressRatio?: number
  onPrime?: boolean
  watch_providers?: WatchProvider[]
  next_episode?: NextEpisode | null
  onQuickMark?: (item: PosterTileItem, e: MouseEvent) => void
  isMarking?: boolean
}

interface Props {
  item: PosterTileItem
}

export function PosterTile({ item }: Props) {
  const { t } = useTranslation()
  const go = () => route(`/title/${item.id}`)
  const providers = item.watch_providers ?? (item.onPrime ? [{ id: 119, name: 'Amazon Prime Video' }] : undefined)

  const handleQuickMarkClick = (e: MouseEvent) => {
    e.stopPropagation()
    e.preventDefault()
    if (!item.isMarking) {
      item.onQuickMark?.(item, e)
    }
  }

  const hasArr = item.sonarr_id != null || item.radarr_id != null

  return (
    <div
      className={s.card}
      onClick={go}
      role="button"
      tabIndex={0}
      onKeyDown={e => e.key === 'Enter' && go()}
    >
      <div className={s.poster}>
        <CoverImage coverUrl={item.cover_url} type={item.type} is_anime={item.is_anime} alt="" />
        <div className={s.badges}>
          <TypeBadge type={item.type} size="sm" radarrId={item.radarr_id} sonarrId={item.sonarr_id} />
          <WatchProviderBadges providers={providers} />
        </div>

        {/* Arr availability badge (top-right) */}
        {hasArr && item.next_episode && (
          <span className={s.arrAvailableBadge}>
            {`S${item.next_episode.season_number.toString().padStart(2, '0')}E${item.next_episode.episode.toString().padStart(2, '0')} ${t('common.dispoBadge')}`}
          </span>
        )}

        {/* Symmetrical +1 Action Button: equal bottom & right offset (10px) */}
        {item.onQuickMark && item.next_episode && (
          <button
            type="button"
            className={clsx(s.quickPlusBtn, item.isMarking && s.quickPlusBtnLoading)}
            onClick={handleQuickMarkClick}
            disabled={item.isMarking}
            aria-label={`Mark S${item.next_episode.season_number} E${item.next_episode.episode} as watched`}
            title={t('common.markNextWatched')}
          >
            {item.isMarking ? (
              <span className={s.quickMarkSpinner} aria-hidden="true" />
            ) : (
              '+1'
            )}
          </button>
        )}

        {item.progressRatio !== undefined && (
          <div className={s.progressBar}>
            <div className={s.progressFill} style={{ width: `${item.progressRatio * 100}%` }} />
          </div>
        )}
        {item.progressRatio === undefined && !hasArr && (
          <span className={`${s.badge} ${s[`badge_${item.sublabelVariant ?? 'default'}`]}`}>
            {item.sublabel}
          </span>
        )}
      </div>
      <div className={s.info}>
        <span className={s.name}>{item.name}</span>
        {item.sublabel && (
          <span className={s.ep}>{item.sublabel}</span>
        )}
      </div>
    </div>
  )
}
