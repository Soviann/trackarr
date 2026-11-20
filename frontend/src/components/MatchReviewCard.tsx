import { route } from 'preact-router'
import type { Title } from '../types'
import { apiFetch } from '../api'
import { getName } from '../utils'
import { routeTo } from '../routes'
import { useTranslation, type TranslationKey } from '../i18n'
import { CoverImage } from './CoverImage'
import s from './MatchReviewCard.module.css'

interface MatchReviewCardProps {
  title: Title
  onUpdate: () => void
}

const matchSourceKeys: Record<string, TranslationKey> = {
  plex_ids: 'matchReview.sourcePlexIds',
  crossref: 'matchReview.sourceCrossref',
  tmdb_search: 'matchReview.sourceTmdbSearch',
  anilist_search: 'matchReview.sourceAnilistSearch',
  gemini_fuzzy: 'matchReview.sourceGeminiFuzzy',
  manual: 'matchReview.sourceManual',
  none: 'matchReview.sourceNone',
}

const STATUS_COLORS: Record<string, string> = {
  unconfirmed: 'var(--status-crit)',
  default: 'var(--accent)',
}

export function MatchReviewCard({ title, onUpdate }: MatchReviewCardProps) {
  const { t } = useTranslation()
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
  const sourceLabel = title.match_source
    ? (matchSourceKeys[title.match_source] ? t(matchSourceKeys[title.match_source]) : title.match_source)
    : null

  const buildStatusText = () => {
    const source = sourceLabel ?? (isUnconfirmed ? t('matchReview.unconfirmedMatch') : t('matchReview.pendingReview'))
    if (isUnconfirmed && !hasAnyID) return t('matchReview.suffixKeepOrFix', { source })
    if (isUnconfirmed && hasAnyID) return t('matchReview.suffixPleaseVerify', { source })
    if (!isUnconfirmed) return t('matchReview.suffixAiVerified', { source })
    return source
  }

  // Show original title if it differs from the resolved name
  const showOriginalTitle = title.original_title && title.original_title !== name

  return (
    <div
      onClick={() => route(routeTo.title(title.id))}
      className={s.card}
      style={{ '--status-color': statusColor }}
    >
      <div className={s.layout}>
        <CoverImage
          coverUrl={title.cover_url}
          type={title.type}
          is_anime={title.is_anime}
          alt={name}
          className={s.cover}
          iconSize="20px"
        />
        <div className={s.body}>
          <div className={s.name}>{name}</div>
          <div className={s.meta}>
            {title.type} · {title.year}
          </div>

          {/* Original title comparison */}
          {showOriginalTitle && (
            <div className={s.originalTitle}>
              {t('matchReview.originalComparison', { original: title.original_title ?? '', name })}
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
          {hasAnyID ? t('common.confirm') : t('matchReview.keepAsIs')}
        </button>
        <button
          onClick={(e: Event) => { e.stopPropagation(); route(`${routeTo.adminValidate()}?q=${encodeURIComponent(title.original_title ?? name)}&id=${title.id}`) }}
          className={s.btnFix}
        >
          {t('matchReview.fixMatch')}
        </button>
      </div>
    </div>
  )
}
