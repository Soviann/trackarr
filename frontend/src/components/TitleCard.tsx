import { memo } from 'preact/compat'
import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import clsx from 'clsx'
import type { Title } from '../types'
import { apiFetch } from '../api'
import { getName, getTypeLabel, formatSortCaption } from '../utils'
import { useTitleStore } from '../store'
import { routeTo } from '../routes'
import { CoverImage } from './CoverImage'
import { StatusBadge } from './StatusBadge'
import { ArrBadge } from './ArrBadge'
import { TypeBadge } from './TypeBadge'
import { WatchProviderBadges } from './WatchProviderBadges'
import s from './TitleCard.module.css'

interface TitleCardProps {
  title: Title
  onUpdate?: () => void
  /** When false, the meta row falls back to type+year regardless of the active sort. */
  showSortCaption?: boolean
}

function getProgress(title: Title) {
  const seasons = title.seasons ?? []
  const ne = title.next_episode

  // Find the relevant season: the one with the next unwatched episode, or the latest with episodes
  let currentSeason = ne
    ? seasons.find((s) => s.season_number === ne.season_number)
    : null
  if (!currentSeason) {
    currentSeason = seasons
      .filter((s) => (s.episode_count ?? (s.episodes ?? []).length) > 0)
      .sort((a, b) => b.season_number - a.season_number)[0] ?? seasons[0]
  }
  if (!currentSeason) return null

  const watched = currentSeason.watched_count ?? (currentSeason.episodes ?? []).filter((e) => e.watched).length
  const total = currentSeason.total_episodes ?? currentSeason.episode_count ?? (currentSeason.episodes ?? []).length

  return { season: currentSeason, watched, total }
}

export const TitleCard = memo(function TitleCard({ title, onUpdate, showSortCaption = true }: TitleCardProps) {
  const sortField = useTitleStore(s => s.sort.field)
  const sortCaption = showSortCaption ? formatSortCaption(title, sortField) : null
  const [toggling, setToggling] = useState(false)
  const name = getName(title)
  const typeLabel = getTypeLabel(title.type)

  const progress = title.type !== 'movie' ? getProgress(title) : null
  const season = progress?.season
  const watched = progress?.watched ?? 0
  const total = progress?.total ?? 0
  const ne = title.next_episode ?? null
  const pct = total > 0 ? (watched / total) * 100 : 0

  const handleQuickMark = async (e: Event) => {
    e.stopPropagation()
    if (!ne || toggling) return
    setToggling(true)
    try {
      await apiFetch(`/titles/${title.id}/episodes/${ne.id}`, { method: 'PATCH' })
      onUpdate?.()
    } catch (err) {
      console.error('Failed to mark episode:', err)
    } finally {
      setToggling(false)
    }
  }

  return (
    <a href={routeTo.title(title.id)} onClick={(e) => { e.preventDefault(); route(routeTo.title(title.id)) }} className={s.card}>
      {/* Cover */}
      <div className={s.cover}>
        <CoverImage
          coverUrl={title.cover_url}
          type={title.type}
          is_anime={title.is_anime}
          alt={name}
          className={s.coverImage}
          iconSize="20px"
        />
        <div className={s.typeBadge}>
          <TypeBadge type={title.type} size="sm" radarrId={title.radarr_id} sonarrId={title.sonarr_id} />
        </div>
      </div>

      {/* Info */}
      <div className={s.info}>
        <div className={s.name}>{name}</div>
        <div className={s.meta}>
          {sortCaption ? (
            <span className={s.sortCaption}>{sortCaption}</span>
          ) : (
            <>{typeLabel} · {title.year}</>
          )}
          <span className={s.statusBadge}>
            <StatusBadge status={title.status} />
            <ArrBadge type={title.type} radarrId={title.radarr_id} sonarrId={title.sonarr_id} />
            <WatchProviderBadges providers={title.watch_providers} />
          </span>
        </div>
        {season && (
          <>
            <div className={s.progressTrack}>
              <div className={s.progressFill} style={{ width: `${pct}%` }} />
            </div>
            <div className={s.progressLabel}>
              S{season.season_number} · {watched}/{total}
            </div>
          </>
        )}
      </div>

      {/* Quick mark badge */}
      {ne && (
        <button
          type="button"
          onClick={handleQuickMark}
          disabled={toggling}
          aria-label={`Mark E${ne.episode} as watched`}
          className={clsx(s.badge, toggling ? s.badgeToggling : s.badgeDefault)}
        >
          <span className={clsx(s.badgeLabel, ne.episode >= 10 ? s.badgeLabelSmall : s.badgeLabelLarge)}>
            E{ne.episode}
          </span>
        </button>
      )}
    </a>
  )
})
