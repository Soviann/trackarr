import { route } from 'preact-router'
import type { ProwlarrRelease, TitleStatus, TitleType } from '../types'
import { routeTo } from '../routes'
import { getCoverUrl } from '../utils'
import { BottomSheet } from './BottomSheet'
import { StatusBadge } from './StatusBadge'
import { TypeBadge } from './TypeBadge'
import { CoverPlaceholder, coverBackground } from './CoverPlaceholder'
import { useTranslation } from '../i18n'
import s from './ReleaseDetailSheet.module.css'

interface ReleaseDetailSheetProps {
  release: ProwlarrRelease | null
  onClose: () => void
  onAdd: (rel: ProwlarrRelease) => Promise<void>
  adding: boolean
  existingTitleId?: number
  existingStatus?: TitleStatus
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const formatted = (bytes / Math.pow(1024, i)).toFixed(i >= 3 ? 1 : 0)
  return `${formatted} ${units[i]}`
}

function formatFullDate(dateStr: string, locale: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  return date.toLocaleDateString(locale === 'fr' ? 'fr-FR' : 'en-US', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function ReleaseDetailSheet({
  release,
  onClose,
  onAdd,
  adding,
  existingTitleId,
  existingStatus,
}: ReleaseDetailSheetProps) {
  const { t, locale } = useTranslation()
  if (!release) return null

  const coverUrl = getCoverUrl(release.poster_url)
  const tmdbUrl = release.tmdb_id
    ? `https://www.themoviedb.org/${release.type === 'movie' ? 'movie' : 'tv'}/${release.tmdb_id}`
    : null
  const imdbUrl = release.imdb_id
    ? (release.imdb_id.startsWith('tt') ? `https://www.imdb.com/title/${release.imdb_id}` : `https://www.imdb.com/title/tt${release.imdb_id}`)
    : null

  return (
    <BottomSheet open={!!release} onClose={onClose} ariaLabel={t('releases.releaseDetails')}>
      <div className={s.sheet}>
        {/* Header with Poster & Essential info */}
        <div className={s.header}>
          <div
            className={s.posterWrap}
            style={{ background: coverBackground(coverUrl, release.type) }}
          >
            {coverUrl ? (
              <div
                className={s.poster}
                style={{ backgroundImage: `url(${coverUrl})` }}
              />
            ) : (
              <CoverPlaceholder type={release.type as TitleType} iconSize="24px" />
            )}
          </div>

          <div className={s.headerInfo}>
            <h2 className={s.title}>{release.clean_title || release.title}</h2>
            <div className={s.badgesRow}>
              {release.year > 0 && <span className={s.year}>{release.year}</span>}
              {existingStatus && <StatusBadge status={existingStatus} />}
              <TypeBadge type={release.type as TitleType} size="sm" />
            </div>

            <div className={s.metaList}>
              <div className={s.metaItem}>
                <span>{t('releases.size')}:</span>
                <strong>{formatBytes(release.size)}</strong>
              </div>
              <div className={s.metaItem}>
                <span>{t('releases.published')}:</span>
                <span>{formatFullDate(release.publish_date, locale)}</span>
              </div>
              <div className={s.metaItem}>
                <span>{t('releases.peers')}:</span>
                <span className={s.seeders}>↑ {release.seeders} {t('releases.seeds')}</span>
                <span>·</span>
                <span>↓ {release.leechers} {t('releases.leeches')}</span>
              </div>
            </div>
          </div>
        </div>

        {/* Raw Scene Release Name */}
        <div className={s.section}>
          <div className={s.sectionLabel}>{t('releases.releaseName')} ({release.indexer || 'Prowlarr'})</div>
          <div className={s.rawBox}>
            {release.title}
          </div>
        </div>

        {/* External links */}
        <div className={s.section}>
          <div className={s.sectionLabel}>{t('releases.externalLinks')}</div>
          <div className={s.linksRow}>
            {release.info_url && (
              <a
                href={release.info_url}
                target="_blank"
                rel="noopener noreferrer"
                className={s.linkChip}
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                  <polyline points="15 3 21 3 21 9" />
                  <line x1="10" y1="14" x2="21" y2="3" />
                </svg>
                {t('releases.indexerPage', { indexer: release.indexer || 'Prowlarr' })}
              </a>
            )}

            {tmdbUrl && (
              <a
                href={tmdbUrl}
                target="_blank"
                rel="noopener noreferrer"
                className={s.linkChip}
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                  <polyline points="15 3 21 3 21 9" />
                  <line x1="10" y1="14" x2="21" y2="3" />
                </svg>
                TMDB
              </a>
            )}

            {imdbUrl && (
              <a
                href={imdbUrl}
                target="_blank"
                rel="noopener noreferrer"
                className={s.linkChip}
              >
                <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                  <polyline points="15 3 21 3 21 9" />
                  <line x1="10" y1="14" x2="21" y2="3" />
                </svg>
                IMDb
              </a>
            )}
          </div>
        </div>

        {/* Primary Action Button */}
        <div className={s.actionWrap}>
          {existingTitleId ? (
            <button
              type="button"
              className={s.btnView}
              onClick={() => {
                onClose()
                route(routeTo.title(existingTitleId))
              }}
            >
              {t('releases.viewInTrackarr')}
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="9 18 15 12 9 6" />
              </svg>
            </button>
          ) : (
            <button
              type="button"
              className={s.btnAdd}
              onClick={() => onAdd(release)}
              disabled={adding}
            >
              {adding ? (
                t('releases.adding')
              ) : (
                <>
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                    <line x1="12" y1="5" x2="12" y2="19" />
                    <line x1="5" y1="12" x2="19" y2="12" />
                  </svg>
                  {t('releases.addToLibraryPlan')}
                </>
              )}
            </button>
          )}
        </div>
      </div>
    </BottomSheet>
  )
}
