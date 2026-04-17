import { route } from 'preact-router'
import s from './PosterStrip.module.css'
import { CoverPlaceholder } from './CoverPlaceholder'

interface PosterStripItem {
  id: number
  cover_url: string | null
  name: string
  sublabel: string
  sublabelVariant?: 'default' | 'amber' | 'teal' | 'muted'
  progressRatio?: number
}

interface Props {
  items: PosterStripItem[]
}

export function PosterStrip({ items }: Props) {
  return (
    <div className={s.strip}>
      {items.map(item => (
        <div
          key={item.id}
          className={s.card}
          onClick={() => route(`/titles/${item.id}`)}
          role="button"
          tabIndex={0}
          onKeyDown={e => e.key === 'Enter' && route(`/titles/${item.id}`)}
        >
          <div className={s.poster}>
            {item.cover_url
              ? <img src={`/api/covers/${item.cover_url}`} alt="" role="presentation" />
              : <CoverPlaceholder />}
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
      ))}
    </div>
  )
}
