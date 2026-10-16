import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title, TitleStatus, Settings } from '../types'
import { routeTo } from '../routes'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { formatBingeTime, unwatchedEpisodesCount, totalEpisodes, isUnairedOrTBA, getName } from '../utils'
import { useTranslation } from '../i18n'
import { useUndo } from '../context/UndoContext'
import s from './NextEpisodeHero.module.css'

interface NextEpisodeHeroProps {
  title: Title
  onEpisodeToggle: (episodeId: number) => Promise<void> | void
  onStatusChange?: (status: TitleStatus) => Promise<void> | void
}

export function NextEpisodeHero({ title, onEpisodeToggle, onStatusChange }: NextEpisodeHeroProps) {
  const { t } = useTranslation()
  const { showUndo } = useUndo()
  const { data: settings } = useApi<Settings>('/settings')
  const [busy, setBusy] = useState(false)

  if (title.status === 'dropped') {
    return null
  }

  // Saga smart next-up: if completed or caught up, check for next chronological relation
  if (title.status === 'completed' || title.caught_up) {
    const sagaEnabled = settings?.saga_chronological_next_up !== false
    if (!sagaEnabled || !title.relations || title.relations.length === 0) {
      return null
    }

    const sortedRelations = [...title.relations].sort((a, b) => {
      const sA = a.season_number ?? 9999
      const sB = b.season_number ?? 9999
      if (sA !== sB) return sA - sB
      if (a.sort_order !== b.sort_order) return a.sort_order - b.sort_order
      const yA = a.year ?? 9999
      const yB = b.year ?? 9999
      if (yA !== yB) return yA - yB
      return (a.title || '').localeCompare(b.title || '')
    })

    const nextRel = sortedRelations.find((r) => r.matched_status !== 'completed')
    if (!nextRel) {
      return null
    }

    const handleAddNextSaga = async () => {
      if (busy) return
      setBusy(true)
      const isRelMovie = nextRel.format === 'MOVIE'
      const payload = {
        type: isRelMovie ? 'movie' : 'series',
        is_anime: nextRel.provider === 'anilist',
        year: nextRel.year,
        status: 'plan_to_watch',
        match_status: 'confirmed',
        names: [{ name: nextRel.title, language: 'en', is_primary: true }],
        cover_url: nextRel.cover_url,
        tmdb_id: nextRel.provider === 'tmdb' ? nextRel.external_id : undefined,
        anilist_id: nextRel.provider === 'anilist' ? nextRel.external_id : undefined,
      }
      try {
        const created = await apiFetch<Title>('/titles', {
          method: 'POST',
          body: JSON.stringify(payload),
        })
        route(routeTo.title(created.id))
      } catch (err) {
        console.error('Failed to add saga sequel to watchlist:', err)
      } finally {
        setBusy(false)
      }
    }

    return (
      <div className={s.container}>
        <div className={s.heroCard}>
          <div className={s.headerRow}>
            <span className={s.label}>
              <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                <polygon points="5 3 19 12 5 21 5 3" />
              </svg>
              {t('hero.continueSaga')}
            </span>
            <div className={s.metaRight}>
              {nextRel.format && (
                <span className={s.availableBadge}>
                  {nextRel.format}
                </span>
              )}
              {nextRel.year && (
                <span className={s.bingeEstimate}>
                  {nextRel.year}
                </span>
              )}
            </div>
          </div>

          <div className={s.episodeInfo}>
            <div className={s.episodeCode}>{nextRel.title}</div>
            {nextRel.romaji_title && nextRel.romaji_title !== nextRel.title && (
              <div className={s.episodeName}>{nextRel.romaji_title}</div>
            )}
          </div>

          {nextRel.matched_title_id != null ? (
            <button
              type="button"
              className={s.markBtn}
              onClick={() => route(routeTo.title(nextRel.matched_title_id!))}
            >
              <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor">
                <polygon points="5 3 19 12 5 21 5 3" />
              </svg>
              {t('hero.viewNextInSaga')}
            </button>
          ) : (
            <button
              type="button"
              className={s.markBtn}
              disabled={busy}
              onClick={handleAddNextSaga}
            >
              <span style={{ fontSize: '16px', lineHeight: 1 }}>+</span>
              {t('hero.addToWatchlist')}
            </button>
          )}
        </div>
      </div>
    )
  }

  const isMovie = title.type === 'movie'
  const isPlanToWatch = title.status === 'plan_to_watch'

  // Default runtime per episode: title runtime if present, or 24m for anime, 45m for standard series
  const epRuntime = title.runtime && title.runtime > 0 ? title.runtime : (title.is_anime ? 24 : 45)

  if (isMovie) {
    if (!isPlanToWatch && title.status !== 'watching') {
      return null
    }

    const movieDuration = title.runtime && title.runtime > 0 ? title.runtime : 120

    return (
      <div className={s.container}>
        <div className={s.heroCard}>
          <div className={s.headerRow}>
            <span className={s.label}>
              {isPlanToWatch ? t('hero.movieToWatch') : t('hero.movieWatching')}
            </span>
            <div className={s.metaRight}>
              {title.downloaded_at && (
                <span className={s.availableBadge}>
                  ✓ {t('details.available')}
                </span>
              )}
              <span className={s.bingeEstimate}>⏱️ {formatBingeTime(movieDuration)}</span>
            </div>
          </div>
          <button
            type="button"
            className={s.markBtn}
            disabled={busy}
            onClick={async () => {
              if (busy || !onStatusChange) return
              setBusy(true)
              const prevStatus = title.status
              try {
                await onStatusChange('completed')
                showUndo({
                  message: t('undo.movieWatched', { title: getName(title) }),
                  onUndo: async () => {
                    if (onStatusChange && prevStatus) {
                      await onStatusChange(prevStatus)
                    }
                  },
                })
              } finally {
                setBusy(false)
              }
            }}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor">
              <polygon points="5 3 19 12 5 21 5 3" />
            </svg>
            {t('hero.markMovieWatched')}
          </button>
        </div>
      </div>
    )
  }

  // Series / Anime logic
  const sortedSeasons = [...(title.seasons ?? [])].sort((a, b) => a.season_number - b.season_number)

  // Find next unwatched episode
  let nextEpSeasonNumber: number | null = null
  let nextEp = null

  for (const season of sortedSeasons) {
    const sortedEps = [...(season.episodes ?? [])].sort((a, b) => a.episode - b.episode)
    const found = sortedEps.find((e) => !e.watched && !isUnairedOrTBA(e))
    if (found) {
      nextEp = found
      nextEpSeasonNumber = season.season_number
      break
    }
  }

  const unwatchedCount = unwatchedEpisodesCount(title)
  const totalCount = totalEpisodes(title)
  const remainingMinutes = unwatchedCount * epRuntime
  const totalMinutes = totalCount * epRuntime

  // If there's a next episode to watch
  if (nextEp && nextEpSeasonNumber != null) {
    const epCode = `S${nextEpSeasonNumber.toString().padStart(2, '0')}E${nextEp.episode.toString().padStart(2, '0')}`

    const handleMarkNext = async () => {
      if (busy || !nextEp) return
      setBusy(true)
      const targetEp = nextEp
      try {
        await onEpisodeToggle(targetEp.id)
        showUndo({
          message: t('undo.quickMarked', {
            title: getName(title),
            ep: epCode,
          }),
          onUndo: async () => {
            await onEpisodeToggle(targetEp.id)
          },
        })
      } finally {
        setBusy(false)
      }
    }

    return (
      <div className={s.container}>
        <div className={s.heroCard}>
          <div className={s.headerRow}>
            <span className={s.label}>
              <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
                <polygon points="5 3 19 12 5 21 5 3" />
              </svg>
              {t('hero.nextEpisode')}
            </span>
            <div className={s.metaRight}>
              {nextEp.downloaded_at && (
                <span className={s.availableBadge}>
                  ✓ {t('details.available')}
                </span>
              )}
              {unwatchedCount > 0 && (
                <span className={s.bingeEstimate}>
                  ⏱️ {t('hero.bingeRemaining')}{formatBingeTime(remainingMinutes)} ({unwatchedCount} ep.)
                </span>
              )}
            </div>
          </div>

          <div className={s.episodeInfo}>
            <div className={s.episodeCode}>{epCode}</div>
            {nextEp.name && <div className={s.episodeName}>{nextEp.name}</div>}
          </div>

          <button
            type="button"
            className={s.markBtn}
            disabled={busy}
            onClick={handleMarkNext}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" fill="currentColor">
              <polygon points="5 3 19 12 5 21 5 3" />
            </svg>
            {t('hero.markEpisodeWatched', { ep: epCode })}
          </button>
        </div>
      </div>
    )
  }

  // If title is plan to watch with no next episode detected yet (empty season list or not started)
  if (isPlanToWatch && totalCount > 0) {
    return (
      <div className={s.container}>
        <div className={s.planCard}>
          <div className={s.planInfo}>
            <span>⏱️</span>
            <span>
              {t('hero.planEstimatedDuration', {
                time: formatBingeTime(totalMinutes),
                count: totalCount,
              })}
            </span>
          </div>
        </div>
      </div>
    )
  }

  return null
}
