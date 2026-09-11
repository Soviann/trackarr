import { useState, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import type { TitleRelation } from '../types'
import { routeTo } from '../routes'
import { aniListMediaUrl, getCoverUrl } from '../utils'
import { useTranslation } from '../i18n'
import { BottomSheet } from './BottomSheet'
import s from './FranchiseRelationsSection.module.css'

interface FranchiseRelationsSectionProps {
  relations?: TitleRelation[]
  onOpenHistory?: () => void
  initialOpenDrawer?: boolean
}

type FilterCategory = 'all' | 'movies' | 'series' | 'ovas' | 'spinoffs'
type SortOrderType = 'timeline' | 'release'

const DEFAULT_VISIBLE_COUNT = 3

export function FranchiseRelationsSection({
  relations = [],
  onOpenHistory,
  initialOpenDrawer = false,
}: FranchiseRelationsSectionProps) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState<FilterCategory>('all')
  const [sortOrder, setSortOrder] = useState<SortOrderType>('timeline')
  const [isExpanded, setIsExpanded] = useState(false)
  const [showDrawer, setShowDrawer] = useState(initialOpenDrawer)

  const movieCount = useMemo(() => relations.filter((r) => r.format === 'MOVIE').length, [relations])
  const seriesCount = useMemo(() => relations.filter((r) => r.format === 'TV').length, [relations])
  const ovaCount = useMemo(() => relations.filter((r) => ['OVA', 'SPECIAL', 'ONA'].includes(r.format)).length, [relations])
  const spinOffCount = useMemo(() => relations.filter((r) => r.relation_type === 'SPIN_OFF').length, [relations])

  const isMainlyCollection = useMemo(
    () => relations.length > 0 && relations.every((r) => r.provider === 'tmdb' || r.relation_type === 'COLLECTION'),
    [relations]
  )

  const providerLabel = useMemo(() => {
    if (relations.length === 0) return ''
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

  if ((!relations || relations.length === 0) && !onOpenHistory) {
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
  const hasRelations = relations && relations.length > 0

  return (
    <>
      {/* ── Proposal 3C: Combined Hub Bar ── */}
      <div className={s.hubBar}>
        {hasRelations && (
          <button
            type="button"
            className={s.hubRow}
            onClick={() => setShowDrawer(true)}
            aria-label={providerLabel}
          >
            <div className={s.hubRowLeft}>
              <div className={s.iconWrapperFranchise}>
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <circle cx="12" cy="12" r="10" />
                  <path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20" />
                  <path d="M2 12h20" />
                </svg>
              </div>
              <div className={s.hubInfo}>
                <div className={s.hubTitleRow}>
                  <span className={s.hubTitle}>{providerLabel}</span>
                  <span className={s.countBadge}>({relations.length})</span>
                </div>
                <div className={s.hubSubline}>
                  <span className={s.hubProgressText}>
                    {t('franchise.titlesSeen', { seen: seenCount, total: totalCount })}
                  </span>
                  {nextChronologicalTitle && (
                    <>
                      <span className={s.hubDot}>•</span>
                      <span className={s.hubNextText}>
                        {t('franchise.nextToWatch')} {nextChronologicalTitle.title}
                      </span>
                    </>
                  )}
                </div>
              </div>
            </div>

            <div className={s.hubRowRight}>
              <div className={s.hubProgressBarWrap}>
                <span className={s.hubProgressPct}>{progressPct}%</span>
                <div className={s.hubProgressBar}>
                  <div className={s.hubProgressFill} style={{ width: `${progressPct}%` }} />
                </div>
              </div>
              <span className={s.chevron}>›</span>
            </div>
          </button>
        )}

        {onOpenHistory && (
          <button
            type="button"
            className={s.hubRow}
            onClick={onOpenHistory}
            aria-label={t('details.watchHistory')}
          >
            <div className={s.hubRowLeft}>
              <div className={s.iconWrapperHistory}>
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                >
                  <circle cx="12" cy="12" r="10" />
                  <polyline points="12 6 12 12 16 14" />
                </svg>
              </div>
              <div className={s.hubInfo}>
                <div className={s.hubTitleRow}>
                  <span className={s.hubTitle}>{t('details.watchHistory')}</span>
                </div>
                <div className={s.hubSubline}>
                  <span className={s.hubHistorySubtext}>
                    {t('details.watchHistorySubtitle')}
                  </span>
                </div>
              </div>
            </div>

            <div className={s.hubRowRight}>
              <span className={s.chevron}>›</span>
            </div>
          </button>
        )}
      </div>

      {/* ── Slide-up BottomSheet Drawer ── */}
      {hasRelations && (
        <BottomSheet
          open={showDrawer}
          onClose={() => setShowDrawer(false)}
          ariaLabel={providerLabel}
        >
          <div className={s.drawerContainer}>
            <div className={s.drawerHeader}>
              <div className={s.drawerTitleGroup}>
                <span className={s.drawerTitle}>
                  <svg
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className={s.drawerIcon}
                  >
                    <circle cx="12" cy="12" r="10" />
                    <path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20" />
                    <path d="M2 12h20" />
                  </svg>
                  {providerLabel}
                </span>
                <span className={s.drawerCountBadge}>({relations.length})</span>
              </div>
              <button
                type="button"
                className={s.drawerCloseBtn}
                onClick={() => setShowDrawer(false)}
                aria-label={t('common.close')}
              >
                ✕
              </button>
            </div>

            <div className={s.drawerBody}>
              {/* Drawer Progress Box */}
              <div className={s.drawerProgressBox}>
                <div className={s.drawerProgressHeader}>
                  <span className={s.drawerProgressLabel}>
                    {t('franchise.titlesSeen', { seen: seenCount, total: totalCount })}
                  </span>
                  <span className={s.drawerProgressPct}>{progressPct}%</span>
                </div>
                <div className={s.sagaProgressBar}>
                  <div className={s.sagaProgressFill} style={{ width: `${progressPct}%` }} />
                </div>
              </div>

              {/* Next Chronological Highlight Card */}
              {nextChronologicalTitle ? (
                <div
                  className={s.nextCard}
                  onClick={() => handleCardClick(nextChronologicalTitle)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      handleCardClick(nextChronologicalTitle)
                    }
                  }}
                >
                  <div className={s.nextCardBadge}>{t('franchise.nextChronological')}</div>
                  <div className={s.nextCardTitle}>
                    {nextChronologicalTitle.title}
                    {nextChronologicalTitle.year ? ` (${nextChronologicalTitle.year})` : ''}
                  </div>
                  <div className={s.nextCardFormat}>
                    {nextChronologicalTitle.format === 'MOVIE'
                      ? t('franchise.movies')
                      : nextChronologicalTitle.format === 'TV'
                      ? t('franchise.series')
                      : nextChronologicalTitle.format}
                    {nextChronologicalTitle.season_number != null ? ` · S${nextChronologicalTitle.season_number}` : ''}
                  </div>
                </div>
              ) : (
                <div className={s.nextCard}>
                  <div className={s.nextCardBadge}>{t('franchise.nextChronological')}</div>
                  <div className={s.nextCardTitle}>{t('franchise.allTitlesSeen')}</div>
                </div>
              )}

              {/* Controls: Sort and Filter */}
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

              {/* List of relationship items */}
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
          </div>
        </BottomSheet>
      )}
    </>
  )
}
