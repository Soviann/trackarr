import { useState, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import type { TitleRelation } from '../types'
import { routeTo } from '../routes'
import { aniListMediaUrl, getCoverUrl } from '../utils'
import { useTranslation } from '../i18n'
import s from './FranchiseRelationsSection.module.css'

interface FranchiseRelationsSectionProps {
  relations: TitleRelation[]
}

type FilterCategory = 'all' | 'movies' | 'series' | 'ovas' | 'spinoffs'
type SortOrderType = 'timeline' | 'release'

const DEFAULT_VISIBLE_COUNT = 3

export function FranchiseRelationsSection({ relations }: FranchiseRelationsSectionProps) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<FilterCategory>('all')
  const [sortOrder, setSortOrder] = useState<SortOrderType>('timeline')
  const [isExpanded, setIsExpanded] = useState(false)

  const movieCount = useMemo(() => relations.filter((r) => r.format === 'MOVIE').length, [relations])
  const seriesCount = useMemo(() => relations.filter((r) => r.format === 'TV').length, [relations])
  const ovaCount = useMemo(() => relations.filter((r) => ['OVA', 'SPECIAL', 'ONA'].includes(r.format)).length, [relations])
  const spinOffCount = useMemo(() => relations.filter((r) => r.relation_type === 'SPIN_OFF').length, [relations])

  const isMainlyCollection = useMemo(() => relations.every((r) => r.provider === 'tmdb' || r.relation_type === 'COLLECTION'), [relations])

  const providerLabel = useMemo(() => {
    const providers = Array.from(new Set(relations.map((r) => r.provider)))
    if (providers.length === 1) {
      if (providers[0] === 'tmdb') return t('franchise.sagaTmdb')
      if (providers[0] === 'tvdb') return t('franchise.tvdbUniverse')
      if (providers[0] === 'anilist') return t('franchise.anilistRelations')
    }
    return isMainlyCollection ? t('franchise.sagaCollection') : t('franchise.universeFranchise')
  }, [relations, isMainlyCollection, t])

  const sortedRelations = useMemo(() => {
    const list = [...relations]
    if (sortOrder === 'release') {
      return list.sort((a, b) => {
        const yearA = a.year ?? 9999
        const yearB = b.year ?? 9999
        if (yearA !== yearB) return yearA - yearB
        return (a.title || '').localeCompare(b.title || '')
      })
    }
    // Default 'timeline': sort by season_number (if any), then sort_order, then year
    return list.sort((a, b) => {
      if (a.season_number != null && b.season_number != null) {
        if (a.season_number !== b.season_number) return a.season_number - b.season_number
      }
      if (a.sort_order !== b.sort_order) return a.sort_order - b.sort_order
      const yearA = a.year ?? 9999
      const yearB = b.year ?? 9999
      return yearA - yearB
    })
  }, [relations, sortOrder])

  const timelineRelations = useMemo(() => {
    const list = [...relations]
    return list.sort((a, b) => {
      if (a.season_number != null && b.season_number != null) {
        if (a.season_number !== b.season_number) return a.season_number - b.season_number
      }
      if (a.sort_order !== b.sort_order) return a.sort_order - b.sort_order
      const yearA = a.year ?? 9999
      const yearB = b.year ?? 9999
      return yearA - yearB
    })
  }, [relations])

  const seenCount = useMemo(
    () => relations.filter((r) => r.matched_status === 'completed').length,
    [relations]
  )
  const totalCount = relations.length
  const progressPct = totalCount > 0 ? Math.round((seenCount / totalCount) * 100) : 0
  const nextChronologicalTitle = useMemo(
    () => timelineRelations.find((r) => r.matched_status !== 'completed'),
    [timelineRelations]
  )

  const filteredRelations = useMemo(() => {
    switch (filter) {
      case 'movies':
        return sortedRelations.filter((r) => r.format === 'MOVIE')
      case 'series':
        return sortedRelations.filter((r) => r.format === 'TV')
      case 'ovas':
        return sortedRelations.filter((r) => ['OVA', 'SPECIAL', 'ONA'].includes(r.format))
      case 'spinoffs':
        return sortedRelations.filter((r) => r.relation_type === 'SPIN_OFF')
      default:
        return sortedRelations
    }
  }, [sortedRelations, filter])

  const visibleRelations = useMemo(() => {
    if (isExpanded || filteredRelations.length <= DEFAULT_VISIBLE_COUNT) {
      return filteredRelations
    }
    return filteredRelations.slice(0, DEFAULT_VISIBLE_COUNT)
  }, [filteredRelations, isExpanded])

  if (!relations || relations.length === 0) {
    return null
  }

  const getExternalUrl = (rel: TitleRelation): string => {
    if (rel.provider === 'tmdb') {
      return rel.format === 'TV'
        ? `https://www.themoviedb.org/tv/${rel.external_id}`
        : `https://www.themoviedb.org/movie/${rel.external_id}`
    }
    if (rel.provider === 'tvdb') {
      return rel.format === 'MOVIE'
        ? `https://thetvdb.com/dereferrer/movies/${rel.external_id}`
        : `https://thetvdb.com/dereferrer/series/${rel.external_id}`
    }
    return aniListMediaUrl(rel.external_id)
  }

  const getProviderName = (provider: string): string => {
    if (provider === 'tmdb') return 'TMDB'
    if (provider === 'tvdb') return 'TheTVDB'
    return 'AniList'
  }

  const handleCardClick = (rel: TitleRelation) => {
    if (rel.matched_title_id != null) {
      route(routeTo.title(rel.matched_title_id))
    } else {
      const extUrl = getExternalUrl(rel)
      route(`/admin/validate?q=${encodeURIComponent(extUrl)}&name=${encodeURIComponent(rel.title)}`)
    }
  }

  const remainingCount = filteredRelations.length - DEFAULT_VISIBLE_COUNT

  return (
    <div className={s.card}>
      {/* 4.1 SAGA / FRANCHISE TRACKER SECTION BY TITLES */}
      <div className={s.sagaTrackerCard}>
        <div className={s.sagaHeader}>
          <span className={s.sagaTitle}>
            <svg
              width="16"
              height="16"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
              className={s.sagaIcon}
            >
              <circle cx="12" cy="12" r="10" />
              <path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20" />
              <path d="M2 12h20" />
            </svg>
            {isMainlyCollection ? t('franchise.sagaCollection') : t('franchise.universeFranchise')}
          </span>
          <span className={s.sagaProgressCount}>
            {t('franchise.titlesSeen', { seen: seenCount, total: totalCount })}
          </span>
        </div>

        <div className={s.sagaProgressBar}>
          <div className={s.sagaProgressFill} style={{ width: `${progressPct}%` }} />
        </div>

        <div className={s.sagaNextBox}>
          <span className={s.sagaNextLabel}>{t('franchise.nextChronological')}</span>
          <span className={s.sagaNextTitle}>
            {nextChronologicalTitle
              ? `${nextChronologicalTitle.title}${nextChronologicalTitle.year ? ` (${nextChronologicalTitle.year})` : ''}`
              : t('franchise.allTitlesSeen')}
          </span>
        </div>

        <div className={s.sagaTitlesStrip}>
          {timelineRelations.map((rel) => {
            const isSeen = rel.matched_status === 'completed'
            const isNext = !isSeen && rel.id === nextChronologicalTitle?.id
            return (
              <button
                key={rel.id || `${rel.provider}-${rel.external_id}`}
                type="button"
                className={`${s.sagaTitleChip} ${isSeen ? s.seen : ''} ${isNext ? s.next : ''}`}
                onClick={() => handleCardClick(rel)}
                title={rel.title}
              >
                {isSeen ? '✓ ' : isNext ? '▶ ' : ''}
                {rel.title}
                {rel.year ? ` (${rel.year})` : ''}
              </button>
            )
          })}
        </div>
      </div>

      <div className={s.cardHeader}>
        <div className={s.cardLabelWrap}>
          <span className={s.cardLabel}>
            {isMainlyCollection ? t('franchise.sagaCollection') : t('franchise.universeFranchise')}
          </span>
          <span className={s.providerBadge}>{providerLabel}</span>
          <span className={s.countBadge}>({relations.length})</span>
        </div>

        <div className={s.controlsArea}>
          {relations.length > 1 && (
            <div className={s.sortToggle} role="group" aria-label="Sort">
              <button
                type="button"
                className={`${s.sortBtn} ${sortOrder === 'timeline' ? s.sortBtnActive : ''}`}
                onClick={() => setSortOrder('timeline')}
                title={t('franchise.sortTimeline')}
              >
                {t('franchise.sortTimeline')}
              </button>
              <button
                type="button"
                className={`${s.sortBtn} ${sortOrder === 'release' ? s.sortBtnActive : ''}`}
                onClick={() => setSortOrder('release')}
                title={t('franchise.sortRelease')}
              >
                {t('franchise.sortRelease')}
              </button>
            </div>
          )}

          <div className={s.filterTabs}>
            <button
              type="button"
              className={`${s.filterBtn} ${filter === 'all' ? s.filterBtnActive : ''}`}
              onClick={() => setFilter('all')}
            >
              {t('franchise.all')} ({relations.length})
            </button>
            {movieCount > 0 && (
              <button
                type="button"
                className={`${s.filterBtn} ${filter === 'movies' ? s.filterBtnActive : ''}`}
                onClick={() => setFilter('movies')}
              >
                {t('franchise.movies')} ({movieCount})
              </button>
            )}
            {seriesCount > 0 && (
              <button
                type="button"
                className={`${s.filterBtn} ${filter === 'series' ? s.filterBtnActive : ''}`}
                onClick={() => setFilter('series')}
              >
                {t('franchise.series')} ({seriesCount})
              </button>
            )}
            {ovaCount > 0 && (
              <button
                type="button"
                className={`${s.filterBtn} ${filter === 'ovas' ? s.filterBtnActive : ''}`}
                onClick={() => setFilter('ovas')}
              >
                {t('franchise.ovas')} ({ovaCount})
              </button>
            )}
            {spinOffCount > 0 && (
              <button
                type="button"
                className={`${s.filterBtn} ${filter === 'spinoffs' ? s.filterBtnActive : ''}`}
                onClick={() => setFilter('spinoffs')}
              >
                {t('franchise.spinoffs')} ({spinOffCount})
              </button>
            )}
          </div>
        </div>
      </div>

      <div className={s.grid}>
        {visibleRelations.map((rel) => {
          const isWatched = rel.matched_status === 'completed'
          const isMatched = rel.matched_title_id != null

          let positionLabel =
            rel.format === 'MOVIE' ? t('franchise.movies') : rel.format === 'TV' ? t('franchise.series') : rel.format
          if (rel.season_number != null) {
            positionLabel += ` · S${rel.season_number}`
          } else if (rel.relation_type === 'PREQUEL') {
            positionLabel = t('franchise.prequel')
          } else if (rel.relation_type === 'SEQUEL') {
            positionLabel = t('franchise.sequel')
          } else if (rel.relation_type === 'SPIN_OFF') {
            positionLabel = t('franchise.spinoffs')
          } else if (rel.relation_type === 'COLLECTION') {
            positionLabel = t('franchise.saga')
          }

          const extUrl = getExternalUrl(rel)
          const providerName = getProviderName(rel.provider)

          return (
            <div
              key={rel.id || `${rel.provider}-${rel.external_id}`}
              className={s.itemCard}
              onClick={() => handleCardClick(rel)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  handleCardClick(rel)
                }
              }}
            >
              <div className={s.cover}>
                {getCoverUrl(rel.cover_url) ? (
                  <img src={getCoverUrl(rel.cover_url)!} alt={rel.title} className={s.coverImg} loading="lazy" />
                ) : (
                  <div className={s.coverFallback}>
                    {rel.format === 'MOVIE' ? '🎬' : rel.format === 'TV' ? '📺' : '🎞'}
                  </div>
                )}
              </div>

              <div className={s.itemBody}>
                <div className={s.itemTop}>
                  <span className={s.relTag}>{positionLabel}</span>
                  {isMatched ? (
                    <span className={isWatched ? s.statusBadgeWatched : s.statusBadgeUnwatched}>
                      {isWatched ? `✓ ${t('franchise.watchedTrackarr')}` : t('franchise.planToWatch')}
                    </span>
                  ) : (
                    <span className={s.statusBadgeAdd}>{t('franchise.addMissing')}</span>
                  )}
                </div>

                <div className={s.itemTitle} title={rel.title}>
                  {rel.title}
                </div>

                <div className={s.itemMeta}>
                  {rel.year && <span>{rel.year}</span>}
                  {rel.duration && <span>· {rel.duration} min</span>}
                  {rel.score != null && <span style={{ color: '#22d3ee' }}>★ {rel.score}%</span>}
                  {!isMatched && (
                    <a
                      href={extUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={s.extLink}
                      onClick={(e) => e.stopPropagation()}
                      title={t('franchise.seeOnProvider', { provider: providerName })}
                    >
                      <span>{providerName}</span>
                      <span>↗</span>
                    </a>
                  )}
                </div>
              </div>
            </div>
          )
        })}
      </div>

      {filteredRelations.length > DEFAULT_VISIBLE_COUNT && (
        <button
          type="button"
          className={s.expandToggle}
          onClick={() => setIsExpanded(!isExpanded)}
        >
          {isExpanded
            ? t('franchise.showLess')
            : t('franchise.showMore', { count: Math.max(0, remainingCount) })}
        </button>
      )}
    </div>
  )
}
