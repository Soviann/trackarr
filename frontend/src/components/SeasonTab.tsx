import clsx from 'clsx'
import { useEffect, useRef } from 'preact/hooks'
import type { Season } from '../types'
import { colors } from '../theme'
import s from './SeasonTab.module.css'

interface SeasonTabProps {
  season: Season
  active: boolean
  onClick: () => void
}

export function SeasonTab({ season, active, onClick }: SeasonTabProps) {
  const eps = season.episodes ?? []
  const watched = eps.filter((e) => e.watched).length
  const total = season.total_episodes ?? eps.length
  const allWatched = total > 0 && watched >= total
  const ref = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    if (active) {
      ref.current?.scrollIntoView({ inline: 'center', block: 'nearest', behavior: 'instant' })
    }
  }, [active])

  if (active) {
    return (
      <button ref={ref} onClick={onClick} className={clsx(s.tab, s.tabActive)}>
        <span className={s.labelActive}>S{season.season_number}</span>
        <span className={s.countActive}>{watched}/{total}</span>
      </button>
    )
  }

  return (
    <button onClick={onClick} className={clsx(s.tab, s.tabInactive)}>
      {allWatched && (
        <svg width="12" height="12" viewBox="0 0 24 24" fill={colors.accentGreen} stroke="none">
          <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z" />
        </svg>
      )}
      <span className={s.labelInactive}>S{season.season_number}</span>
      {season.my_rating != null && (
        <span className={s.rating}>��� {season.my_rating}</span>
      )}
    </button>
  )
}
