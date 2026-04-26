import s from './SectionRow.module.css'
import type { Title } from '../types'

interface SectionRowProps {
  label: string
  subText: string
  posters: Pick<Title, 'id' | 'cover_url' | 'type'>[]
  onClick: () => void
}

export function SectionRow({ label, subText, posters, onClick }: SectionRowProps) {
  const peek = posters.slice(0, 3)
  return (
    <button type="button" className={s.row} onClick={onClick}>
      <div className={s.text}>
        <div className={s.label}>{label}</div>
        <div className={s.sub}>{subText}</div>
      </div>
      <div className={s.peek}>
        {peek.map((p, i) => (
          <div
            key={p.id}
            className={s.miniPoster}
            style={{
              backgroundImage: p.cover_url ? `url(/api/covers/${p.cover_url})` : undefined,
              transform: `translateX(${i * -8}px)`,
              zIndex: peek.length - i,
            }}
          />
        ))}
      </div>
      <svg className={s.chev} width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="9 18 15 12 9 6" />
      </svg>
    </button>
  )
}
