import clsx from 'clsx'
import { useTranslation } from '../i18n'
import type { TitleType } from '../types'
import s from './ExternalLinks.module.css'

export interface ExternalLinksProps {
  tmdbId?: number | null
  anilistId?: number | null
  imdbId?: string | null
  type?: TitleType
  className?: string
}

export function ExternalLinks({
  tmdbId,
  anilistId,
  imdbId,
  type = 'movie',
  className,
}: ExternalLinksProps) {
  const { t } = useTranslation()

  const hasTmdb = tmdbId != null && tmdbId > 0
  const hasAnilist = anilistId != null && anilistId > 0
  const hasImdb = Boolean(imdbId && imdbId.trim() !== '')

  if (!hasTmdb && !hasAnilist && !hasImdb) {
    return null
  }

  const tmdbUrl = hasTmdb
    ? `https://www.themoviedb.org/${type === 'movie' ? 'movie' : 'tv'}/${tmdbId}`
    : null
  const anilistUrl = hasAnilist
    ? `https://anilist.co/anime/${anilistId}`
    : null
  const trimmedImdb = imdbId?.trim()
  const imdbUrl = trimmedImdb
    ? `https://www.imdb.com/title/${trimmedImdb.toLowerCase().startsWith('tt') ? trimmedImdb.toLowerCase() : `tt${trimmedImdb}`}/`
    : null

  return (
    <div className={clsx(s.container, className)}>
      {tmdbUrl && (
        <a
          href={tmdbUrl}
          target="_blank"
          rel="noopener noreferrer"
          className={clsx(s.badge, s.tmdb)}
          onClick={(e) => e.stopPropagation()}
          aria-label={t('common.openExternal', { source: 'TMDB' })}
          title={t('common.openExternal', { source: 'TMDB' })}
        >
          TMDB ↗
        </a>
      )}
      {anilistUrl && (
        <a
          href={anilistUrl}
          target="_blank"
          rel="noopener noreferrer"
          className={clsx(s.badge, s.anilist)}
          onClick={(e) => e.stopPropagation()}
          aria-label={t('common.openExternal', { source: 'AniList' })}
          title={t('common.openExternal', { source: 'AniList' })}
        >
          AniList ↗
        </a>
      )}
      {imdbUrl && (
        <a
          href={imdbUrl}
          target="_blank"
          rel="noopener noreferrer"
          className={clsx(s.badge, s.imdb)}
          onClick={(e) => e.stopPropagation()}
          aria-label={t('common.openExternal', { source: 'IMDb' })}
          title={t('common.openExternal', { source: 'IMDb' })}
        >
          IMDb ↗
        </a>
      )}
    </div>
  )
}
