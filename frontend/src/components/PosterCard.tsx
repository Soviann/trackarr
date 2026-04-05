import { route } from 'preact-router'
import type { Title } from '../types'
import { getName } from '../utils'
import { CoverPlaceholder, coverBackground } from './CoverPlaceholder'
import s from './PosterCard.module.css'

interface PosterCardProps {
  title: Title
}

export function PosterCard({ title }: PosterCardProps) {
  const name = getName(title)

  return (
    <div onClick={() => route(`/title/${title.id}`)} className={s.card}>
      <div
        className={s.poster}
        style={{ background: coverBackground(title.cover_url, title.type) }}
      >
        {!title.cover_url && <CoverPlaceholder type={title.type} />}
        <div className={s.labelOverlay}>
          <div className={s.label}>{name}</div>
        </div>
      </div>
    </div>
  )
}
