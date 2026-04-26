import { route } from 'preact-router'
import type { TitleType } from '../types'
import s from './PosterTile.module.css'
import { CoverPlaceholder } from './CoverPlaceholder'

export interface PosterTileItem {
  id: number
  type: TitleType
  cover_url: string | null
  name: string
  sublabel: string
  sublabelVariant?: 'default' | 'amber' | 'teal' | 'muted'
  progressRatio?: number
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
        {item.cover_url
          ? <img src={`/api/covers/${item.cover_url}`} alt="" role="presentation" />
          : <CoverPlaceholder type={item.type} />}
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
