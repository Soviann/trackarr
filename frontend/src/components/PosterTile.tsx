import { route } from 'preact-router'
import type { TitleType } from '../types'
import s from './PosterTile.module.css'
import { CoverImage } from './CoverImage'
import { PrimeBadge } from './PrimeBadge'
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
}

interface Props {
  item: PosterTileItem
}

export function PosterTile({ item }: Props) {
  const go = () => route(`/title/${item.id}`)
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
          {item.onPrime && <PrimeBadge />}
        </div>
        {item.progressRatio !== undefined && (
          <div className={s.progressBar}>
            <div className={s.progressFill} style={{ width: `${item.progressRatio * 100}%` }} />
          </div>
        )}
        {item.progressRatio === undefined && (
          <span className={`${s.badge} ${s[`badge_${item.sublabelVariant ?? 'default'}`]}`}>
            {item.sublabel}
          </span>
        )}
      </div>
      <div className={s.info}>
        <span className={s.name}>{item.name}</span>
        {item.progressRatio !== undefined && (
          <span className={s.ep}>{item.sublabel}</span>
        )}
      </div>
    </div>
  )
}
