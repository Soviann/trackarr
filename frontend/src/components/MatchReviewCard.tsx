import { route } from 'preact-router'
import type { Title } from '../types'
import { apiFetch } from '../api'
import { getName } from '../utils'
import { CoverPlaceholder, coverBackground } from './CoverPlaceholder'
import s from './MatchReviewCard.module.css'

interface MatchReviewCardProps {
  title: Title
  onUpdate: () => void
}

const matchSourceLabels: Record<string, string> = {
  plex_ids: 'Matched via Plex metadata',
  crossref: 'Matched via cross-reference DB',
  tmdb_search: 'Matched via TMDB search',
  anilist_search: 'Matched via AniList',
  gemini_fuzzy: 'Matched via AI fuzzy resolution',
  manual: 'Manual entry',
  none: 'No automatic match found',
}

const STATUS_COLORS: Record<string, string> = {
  unconfirmed: 'var(--status-crit)',
  default: 'var(--accent)',
}

export function MatchReviewCard({ title, onUpdate }: MatchReviewCardProps) {
  const name = getName(title)
  const isUnconfirmed = title.match_status === 'unconfirmed'
  const statusColor = isUnconfirmed ? STATUS_COLORS.unconfirmed : STATUS_COLORS.default
  const hasAnyID = !!(title.imdb_id || title.tmdb_id || title.tvdb_id || title.anilist_id || title.simkl_id)

  const handleConfirm = async (e: Event) => {
    e.stopPropagation()
    await apiFetch(`/titles/${title.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ match_status: 'confirmed' }),
    })
    onUpdate()
  }

  const simklSection = title.type === 'movie' ? 'movies' : title.is_anime ? 'anime' : 'tv'
  const idChips = [
    title.simkl_id && { label: 'Simkl', value: String(title.simkl_id), href: title.simkl_slug ? `https://simkl.com/${simklSection}/${title.simkl_id}/${title.simkl_slug}` : `https://simkl.com/${simklSection}/${title.simkl_id}` },
    title.imdb_id && { label: 'IMDb', value: title.imdb_id, href: `https://www.imdb.com/title/${title.imdb_id}/` },
    title.tmdb_id && { label: 'TMDB', value: String(title.tmdb_id), href: `https://www.themoviedb.org/${title.type === 'movie' ? 'movie' : 'tv'}/${title.tmdb_id}` },
    title.tvdb_id && { label: 'TVDB', value: String(title.tvdb_id) },
    title.anilist_id && { label: 'AniList', value: String(title.anilist_id), href: `https://anilist.co/anime/${title.anilist_id}` },
  ].filter(Boolean) as { label: string; value: string; href?: string }[]

  // Build contextual explanation
  const sourceLabel = title.match_source ? (matchSourceLabels[title.match_source] ?? title.match_source) : null

  const buildStatusText = () => {
    const source = sourceLabel ?? (isUnconfirmed ? 'Unconfirmed match' : 'Pending review')
    if (isUnconfirmed && !hasAnyID) return `${source} — keep as-is or fix match`
    if (isUnconfirmed && hasAnyID) return `${source} — please verify`
    if (!isUnconfirmed) return `${source} — AI-verified, confirm?`
    return source
  }

  // Show original title if it differs from the resolved name
  const showOriginalTitle = title.original_title && title.original_title !== name

  return (
    <div
      onClick={() => route(`/title/${title.id}`)}
      className={s.card}
      style={{ '--status-color': statusColor }}
    >
      <div className={s.layout}>
        <div
          className={s.cover}
          style={{ background: coverBackground(title.cover_url, title.type) }}
        >
          {!title.cover_url && <CoverPlaceholder type={title.type} iconSize="20px" />}
        </div>
        <div className={s.body}>
          <div className={s.name}>{name}</div>
          <div className={s.meta}>
            {title.type} · {title.year}
          </div>

          {/* Original title comparison */}
          {showOriginalTitle && (
            <div className={s.originalTitle}>
              Plex: "{title.original_title}" → "{name}"
            </div>
          )}

          {/* ID chips */}
          {idChips.length > 0 && (
            <div className={s.chips}>
              {idChips.map((chip) =>
                chip.href ? (
                  <a key={chip.label} className={s.chip} href={chip.href} target="_blank" rel="noopener noreferrer" onClick={(e: Event) => e.stopPropagation()}>
                    {chip.label}: {chip.value}
                  </a>
                ) : (
                  <span key={chip.label} className={s.chip}>
                    {chip.label}: {chip.value}
                  </span>
                )
              )}
            </div>
          )}

          {/* Match source + contextual explanation */}
          <div className={s.statusText}>
            {buildStatusText()}
          </div>
        </div>
      </div>

      {/* Actions */}
      <div className={s.actions}>
        <button
          onClick={handleConfirm}
          className={s.btnConfirm}
        >
          {hasAnyID ? 'Confirm' : 'Keep as-is'}
        </button>
        <button
          onClick={(e: Event) => { e.stopPropagation(); route(`/admin/validate?q=${encodeURIComponent(title.original_title ?? name)}&id=${title.id}`) }}
          className={s.btnFix}
        >
          Fix match
        </button>
      </div>
    </div>
  )
}
