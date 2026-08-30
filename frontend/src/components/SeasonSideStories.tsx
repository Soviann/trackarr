import { route } from 'preact-router'
import type { TitleRelation } from '../types'
import { routeTo } from '../routes'
import { aniListMediaUrl, getCoverUrl } from '../utils'
import { useTranslation } from '../i18n'
import s from './SeasonSideStories.module.css'

interface SeasonSideStoriesProps {
  seasonNumber: number
  sideStories: TitleRelation[]
  onToggleWatched?: (relation: TitleRelation) => void
}

export function SeasonSideStories({ seasonNumber, sideStories, onToggleWatched }: SeasonSideStoriesProps) {
  const { t } = useTranslation()

  if (!sideStories || sideStories.length === 0) {
    return null
  }

  const formatFormat = (format: string): { label: string; className: string } => {
    switch (format.toUpperCase()) {
      case 'MOVIE':
        return { label: t('franchise.movies'), className: s.formatBadge }
      case 'OVA':
        return { label: t('franchise.ovas'), className: `${s.formatBadge} ${s.formatBadgeOva}` }
      case 'SPECIAL':
        return { label: t('sideStories.special'), className: `${s.formatBadge} ${s.formatBadgeSpecial}` }
      case 'ONA':
        return { label: 'ONA', className: `${s.formatBadge} ${s.formatBadgeSpecial}` }
      default:
        return { label: format, className: s.formatBadge }
    }
  }

  const formatDuration = (minutes?: number | null): string | null => {
    if (!minutes || minutes <= 0) return null
    const h = Math.floor(minutes / 60)
    const m = minutes % 60
    return h > 0 ? `${h}h ${m.toString().padStart(2, '0')}m` : `${m} min`
  }

  return (
    <div className={s.container}>
      <div className={s.header}>
        <div className={s.headerTitle}>
          <span>🎬</span>
          <span>
            {sideStories.length === 1
              ? t('sideStories.recommendedAfterSeason', { season: seasonNumber })
              : t('sideStories.recommendedAfterSeasonPlural', {
                  season: seasonNumber,
                  count: sideStories.length,
                })}
          </span>
        </div>
        <span className={s.headerTag}>{t('sideStories.chronologicalOrder')}</span>
      </div>

      {sideStories.map((rel) => {
        const fmt = formatFormat(rel.format)
        const dur = formatDuration(rel.duration)
        const isWatched = rel.matched_status === 'completed'
        const isMatched = rel.matched_title_id != null

        return (
          <div key={rel.id || rel.external_id} className={s.card}>
            <div className={s.cardContent}>
              <div className={s.coverWrap}>
                {getCoverUrl(rel.cover_url) ? (
                  <img src={getCoverUrl(rel.cover_url)!} alt={rel.title} className={s.coverImg} loading="lazy" />
                ) : (
                  <div className={s.coverFallback}>
                    {rel.format === 'MOVIE' ? '🎬' : '🎞'}
                  </div>
                )}
              </div>

              <div className={s.info}>
                <div className={s.metaRow}>
                  <span className={fmt.className}>{fmt.label}</span>
                  {rel.year && <span className={s.metaText}>{rel.year}</span>}
                  {dur && <span className={s.metaText}>· {dur}</span>}
                  {rel.score != null && <span className={s.score}>★ {rel.score}%</span>}
                </div>

                <div className={s.title} title={rel.title}>
                  {rel.title}
                </div>

                {rel.overview && <div className={s.overview}>{rel.overview}</div>}
              </div>
            </div>

            <div className={s.actions}>
              {isMatched ? (
                <>
                  <button
                    type="button"
                    className={`${s.statusBtn} ${isWatched ? s.statusWatched : s.statusUnwatched}`}
                    onClick={() => onToggleWatched?.(rel)}
                  >
                    <span>{isWatched ? '✓' : '○'}</span>
                    <span>{isWatched ? t('franchise.watchedTrackarr') : t('franchise.markWatched')}</span>
                  </button>

                  <button
                    type="button"
                    className={s.actionLink}
                    onClick={() => route(routeTo.title(rel.matched_title_id!))}
                  >
                    <span>{rel.format === 'MOVIE' ? t('franchise.viewMovie') : t('franchise.viewTitle')}</span>
                    <span>↗</span>
                  </button>
                </>
              ) : (
                <>
                  <span className={s.unmatchedBadge}>{t('sideStories.notInLibrary')}</span>
                  <button
                    type="button"
                    className={s.addBtn}
                    onClick={() =>
                      route(
                        `/admin/validate?q=${encodeURIComponent(
                          aniListMediaUrl(rel.external_id)
                        )}&name=${encodeURIComponent(rel.title)}`
                      )
                    }
                  >
                    <span>+</span>
                    <span>{t('common.add')}</span>
                  </button>
                  <a
                    href={aniListMediaUrl(rel.external_id)}
                    target="_blank"
                    rel="noopener noreferrer"
                    className={s.actionLink}
                    title={t('franchise.seeOnProvider', { provider: 'AniList' })}
                  >
                    <span>AniList</span>
                    <span>↗</span>
                  </a>
                </>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
