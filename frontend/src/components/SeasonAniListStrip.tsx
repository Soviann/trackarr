import type { AniListPart, Season } from '../types'
import { aniListMediaUrl } from '../utils'
import s from './SeasonAniListStrip.module.css'

interface SeasonAniListStripProps {
  season: Season
  entryName?: string
  onEdit: () => void
}

// Derive the parts list, falling back to the legacy single-id fields so existing
// single-link seasons render unchanged.
function seasonParts(season: Season): AniListPart[] {
  if (season.anilist_parts && season.anilist_parts.length > 0) return season.anilist_parts
  if (season.anilist_id != null) {
    return [{
      external_id: season.anilist_id,
      score: season.anilist_community_score ?? null,
      episode_count: null, start_date: null, sort_order: null,
    }]
  }
  return []
}

export function SeasonAniListStrip({ season, entryName, onEdit }: SeasonAniListStripProps) {
  const parts = seasonParts(season)

  if (parts.length === 0) {
    return (
      <div className={s.stripUnmapped}>
        <span className={s.label}>ANILIST</span>
        <span className={s.unmappedText}>Not mapped for this season</span>
        <button type="button" className={s.linkButton} onClick={onEdit}>Link entry</button>
      </div>
    )
  }

  const multi = parts.length > 1
  return (
    <div className={s.stripMapped}>
      <span className={s.label}>ANILIST</span>
      <div className={s.partList}>
        {parts.map((p, i) => (
          <span key={p.external_id} className={s.partRow}>
            {multi && <span className={s.partTag}>Part {i + 1}</span>}
            <a href={aniListMediaUrl(p.external_id)} target="_blank" rel="noopener noreferrer"
               className={`${s.entryName} ${s.entryLink}`}>
              {multi ? `Part ${i + 1}` : (entryName ?? `S${season.season_number}`)}
            </a>
            {p.score != null && <span className={s.score}>{p.score}%</span>}
          </span>
        ))}
      </div>
      <button type="button" className={s.editButton} onClick={onEdit} aria-label="Edit AniList mapping">✎</button>
    </div>
  )
}
