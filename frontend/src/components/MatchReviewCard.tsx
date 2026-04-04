import { route } from 'preact-router'
import type { Title } from '../types'
import { colors } from '../theme'
import { apiFetch } from '../api'
import { CoverPlaceholder, coverBackground } from './CoverPlaceholder'

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

export function MatchReviewCard({ title, onUpdate }: MatchReviewCardProps) {
  const name = (title.names ?? []).find((n) => n.is_primary)?.name ?? 'Untitled'
  const isUnconfirmed = title.match_status === 'unconfirmed'
  const borderColor = isUnconfirmed ? colors.accentCoral : colors.accentAmber
  const hasAnyID = !!(title.imdb_id || title.tmdb_id || title.tvdb_id || title.anilist_id)

  const handleConfirm = async (e: Event) => {
    e.stopPropagation()
    await apiFetch(`/titles/${title.id}`, {
      method: 'PATCH',
      body: JSON.stringify({ match_status: 'confirmed' }),
    })
    onUpdate()
  }

  const idChips = [
    title.imdb_id && { label: 'IMDb', value: title.imdb_id },
    title.tmdb_id && { label: 'TMDB', value: String(title.tmdb_id) },
    title.tvdb_id && { label: 'TVDB', value: String(title.tvdb_id) },
    title.anilist_id && { label: 'AniList', value: String(title.anilist_id) },
  ].filter(Boolean) as { label: string; value: string }[]

  // Build contextual explanation
  const sourceLabel = title.match_source ? (matchSourceLabels[title.match_source] ?? title.match_source) : null

  const buildStatusText = () => {
    const source = sourceLabel ?? (isUnconfirmed ? 'Unconfirmed match' : 'Pending review')
    if (isUnconfirmed && !hasAnyID) return `${source} — needs manual linking`
    if (isUnconfirmed && hasAnyID) return `${source} — please verify`
    if (!isUnconfirmed) return `${source} — AI-verified, confirm?`
    return source
  }

  // Show original title if it differs from the resolved name
  const showOriginalTitle = title.original_title && title.original_title !== name

  return (
    <div
      onClick={() => route(`/title/${title.id}`)}
      style={{
        background: colors.bgCard,
        borderRadius: '12px',
        padding: '12px',
        border: `1px solid ${borderColor}33`,
        borderLeft: `3px solid ${borderColor}`,
        cursor: 'pointer',
      }}
    >
      <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-start' }}>
        <div style={{
          width: '48px', height: '68px', borderRadius: '6px', flexShrink: 0,
          background: coverBackground(title.cover_url, title.type),
          position: 'relative', overflow: 'hidden',
        }}>
          {!title.cover_url && <CoverPlaceholder type={title.type} iconSize="20px" />}
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div style={{ fontSize: '13px', fontWeight: 600, color: colors.textPrimary }}>{name}</div>
          <div style={{ fontSize: '10px', color: colors.textSecondary, marginTop: '2px' }}>
            {title.type} · {title.year}
          </div>

          {/* Original title comparison */}
          {showOriginalTitle && (
            <div style={{
              fontSize: '10px',
              color: colors.textSecondary,
              marginTop: '4px',
              fontStyle: 'italic',
            }}>
              Plex: "{title.original_title}" → "{name}"
            </div>
          )}

          {/* ID chips */}
          {idChips.length > 0 && (
            <div style={{ display: 'flex', gap: '4px', marginTop: '6px', flexWrap: 'wrap' }}>
              {idChips.map((chip) => (
                <span
                  key={chip.label}
                  style={{
                    fontSize: '9px',
                    color: colors.textSecondary,
                    background: colors.bgSurface,
                    borderRadius: '4px',
                    padding: '2px 6px',
                  }}
                >
                  {chip.label}: {chip.value}
                </span>
              ))}
            </div>
          )}

          {/* Match source + contextual explanation */}
          <div style={{
            fontSize: '10px',
            color: borderColor,
            fontWeight: 500,
            marginTop: '6px',
          }}>
            {buildStatusText()}
          </div>
        </div>
      </div>

      {/* Actions */}
      <div style={{ display: 'flex', gap: '8px', marginTop: '10px' }}>
        <button
          onClick={handleConfirm}
          disabled={!hasAnyID}
          style={{
            flex: 1,
            padding: '8px',
            borderRadius: '8px',
            background: hasAnyID ? `${colors.accentGreen}1F` : `${colors.textMuted}1F`,
            border: `1px solid ${hasAnyID ? colors.accentGreen : colors.textMuted}33`,
            color: hasAnyID ? colors.accentGreen : colors.textMuted,
            fontSize: '11px',
            fontWeight: 600,
            cursor: hasAnyID ? 'pointer' : 'not-allowed',
            opacity: hasAnyID ? 1 : 0.5,
          }}
        >
          Confirm
        </button>
        <button
          onClick={(e: Event) => { e.stopPropagation(); route(`/validate?q=${encodeURIComponent(title.original_title ?? name)}`) }}
          style={{
            flex: 1,
            padding: '8px',
            borderRadius: '8px',
            background: colors.bgSurface,
            border: `1px solid ${colors.borderCard}`,
            color: colors.textSecondary,
            fontSize: '11px',
            fontWeight: 500,
            cursor: 'pointer',
          }}
        >
          Fix match
        </button>
      </div>
    </div>
  )
}
