import { useState } from 'preact/hooks'
import type { Title, TitleStatus } from '../types'
import { formatBingeTime, unwatchedEpisodesCount, totalEpisodes } from '../utils'
import { useTranslation } from '../i18n'
import s from './NextEpisodeHero.module.css'

interface NextEpisodeHeroProps {
  title: Title
  onEpisodeToggle: (episodeId: number) => Promise<void> | void
  onStatusChange?: (status: TitleStatus) => Promise<void> | void
}

export function NextEpisodeHero({ title, onEpisodeToggle, onStatusChange }: NextEpisodeHeroProps) {
  const { t } = useTranslation()
  const [busy, setBusy] = useState(false)

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
            <span className={s.bingeEstimate}>⏱️ {formatBingeTime(movieDuration)}</span>
          </div>
          <button
            type="button"
            className={s.markBtn}
            disabled={busy}
            onClick={async () => {
              if (busy || !onStatusChange) return
              setBusy(true)
              try {
                await onStatusChange('completed')
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
    const found = sortedEps.find((e) => !e.watched)
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
      try {
        await onEpisodeToggle(nextEp.id)
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
            {unwatchedCount > 0 && (
              <span className={s.bingeEstimate}>
                ⏱️ {t('hero.bingeRemaining')}{formatBingeTime(remainingMinutes)} ({unwatchedCount} ep.)
              </span>
            )}
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
