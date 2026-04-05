import { useState } from 'preact/hooks'
import clsx from 'clsx'
import { colors } from '../theme'
import { apiFetch } from '../api'
import type { Episode } from '../types'
import s from './EpisodeRow.module.css'

interface EpisodeRowProps {
  titleId: number
  episode: Episode
  onToggle?: () => void
}

export function EpisodeRow({ titleId, episode, onToggle }: EpisodeRowProps) {
  const [toggling, setToggling] = useState(false)

  const handleToggle = async (e: Event) => {
    e.stopPropagation()
    if (toggling) return
    setToggling(true)
    try {
      await apiFetch(`/titles/${titleId}/episodes/${episode.id}`, { method: 'PATCH' })
      onToggle?.()
    } finally {
      setToggling(false)
    }
  }

  return (
    <div className={clsx(s.row, episode.watched && s.watched)}>
      <div className={s.info}>
        <span className={s.episodeNumber}>E{episode.episode}</span>
        {episode.name && (
          <span className={s.episodeName}>— {episode.name}</span>
        )}
        {episode.air_date && (
          <span className={s.airDate}>{episode.air_date}</span>
        )}
      </div>

      <div onClick={handleToggle} className={s.toggle}>
        {episode.watched ? (
          <svg width="18" height="18" viewBox="0 0 24 24" fill={colors.accentAmber} stroke="none">
            <path d="M20 6L9 17l-5-5 1.41-1.41L9 14.17 18.59 4.58z" />
          </svg>
        ) : (
          <div className={s.checkbox} />
        )}
      </div>
    </div>
  )
}
