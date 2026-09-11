import { useMemo } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import type { EpisodeHistory } from '../types'
import { groupIntoRanges, formatRangeLabel, type EpisodeRangeGroup } from '../utils/episodeRanges'
import { useTranslation } from '../i18n'
import s from './TitleHistory.module.css'

interface Props {
  titleId: number
  onClose: () => void
}

interface SeasonGroup {
  seasonNumber: number | null
  ranges: EpisodeRangeGroup<EpisodeHistory>[]
  mostRecent: string
}

function buildSeasonGroups(episodes: EpisodeHistory[]): SeasonGroup[] {
  // Group by season
  const bySeason = new Map<number | null, EpisodeHistory[]>()
  for (const ep of episodes) {
    const key = ep.season_number ?? null
    const list = bySeason.get(key) ?? []
    list.push(ep)
    bySeason.set(key, list)
  }

  const groups: SeasonGroup[] = []
  for (const [seasonNumber, eps] of bySeason) {
    // Sort by episode_number ASC within each season
    eps.sort((a, b) => (a.episode_number ?? 0) - (b.episode_number ?? 0))
    const ranges = groupIntoRanges(eps)
    const mostRecent = eps.reduce((max, e) =>
      e.last_watched_at > max ? e.last_watched_at : max, eps[0].last_watched_at)
    groups.push({ seasonNumber, ranges, mostRecent })
  }

  // Sort seasons by most recently watched first
  groups.sort((a, b) => b.mostRecent.localeCompare(a.mostRecent))
  return groups
}

export function TitleHistory({ titleId, onClose }: Props) {
  const { t } = useTranslation()
  const { data, loading } = useApi<EpisodeHistory[]>(`/titles/${titleId}/history`)

  const seasonGroups = useMemo(() => data ? buildSeasonGroups(data) : [], [data])

  return (
    <div className={s.container}>
      <div className={s.header}>
        <span className={s.title}>
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            className={s.clockIcon}
          >
            <circle cx="12" cy="12" r="10" />
            <polyline points="12 6 12 12 16 14" />
          </svg>
          {t('details.watchHistory')}
        </span>
        <button className={s.closeBtn} onClick={onClose} aria-label={t('common.close')}>
          ✕
        </button>
      </div>

      <div className={s.content}>
        {loading && <div className={s.loading}>{t('common.loading')}</div>}
        {seasonGroups.map((sg) => (
          <div key={sg.seasonNumber ?? 'movie'}>
            {sg.seasonNumber != null && (
              <div className={s.seasonDivider}>Season {sg.seasonNumber}</div>
            )}
            {sg.ranges.map((range) => {
              const label = formatRangeLabel(range)
              const isSingle = range.items.length === 1
              const displayLabel = isSingle && range.episodeName
                ? `${label} — ${range.episodeName}`
                : label
              const maxDate = range.items.reduce((max, e) =>
                e.last_watched_at > max ? e.last_watched_at : max, range.items[0].last_watched_at)
              return (
                <div key={`${range.startEp}-${range.endEp}`} className={s.row}>
                  <div className={s.info}>
                    <span className={s.epLabel}>{displayLabel}</span>
                    <span className={s.date}>
                      {new Date(maxDate).toLocaleDateString(undefined, {
                        day: 'numeric', month: 'short', year: 'numeric'
                      })}
                    </span>
                  </div>
                  {isSingle && range.items[0].watch_count > 1 && (
                    <span className={s.rewatchBadge}>×{range.items[0].watch_count}</span>
                  )}
                </div>
              )
            })}
          </div>
        ))}
        {!loading && data?.length === 0 && (
          <div className={s.loading}>{t('details.noWatchesRecorded')}</div>
        )}
      </div>
    </div>
  )
}
