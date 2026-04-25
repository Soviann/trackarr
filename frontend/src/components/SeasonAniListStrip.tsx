import type { Season } from '../types'
import s from './SeasonAniListStrip.module.css'

interface SeasonAniListStripProps {
  season: Season
  entryName?: string
  onEdit: () => void
}

export function SeasonAniListStrip({ season, entryName, onEdit }: SeasonAniListStripProps) {
  const isMapped = season.anilist_id != null

  if (!isMapped) {
    return (
      <div className={s.stripUnmapped}>
        <span className={s.label}>ANILIST</span>
        <span className={s.unmappedText}>Not mapped for this season</span>
        <button type="button" className={s.linkButton} onClick={onEdit}>
          Link entry
        </button>
      </div>
    )
  }

  return (
    <div className={s.stripMapped}>
      <span className={s.label}>ANILIST</span>
      <span className={s.entryName}>{entryName ?? `S${season.season_number}`}</span>
      {season.anilist_community_score != null && (
        <span className={s.score}>{season.anilist_community_score}%</span>
      )}
      <button
        type="button"
        className={s.editButton}
        onClick={onEdit}
        aria-label="Edit AniList mapping"
      >
        ✎
      </button>
    </div>
  )
}
