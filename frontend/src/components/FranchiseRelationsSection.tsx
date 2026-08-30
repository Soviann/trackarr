import { useState, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import type { TitleRelation } from '../types'
import { routeTo } from '../routes'
import { aniListMediaUrl, getCoverUrl } from '../utils'
import s from './FranchiseRelationsSection.module.css'

interface FranchiseRelationsSectionProps {
  relations: TitleRelation[]
}

type FilterCategory = 'all' | 'movies' | 'series' | 'ovas' | 'spinoffs'
type SortOrderType = 'timeline' | 'release'

const DEFAULT_VISIBLE_COUNT = 3

export function FranchiseRelationsSection({ relations }: FranchiseRelationsSectionProps) {
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
      if (providers[0] === 'tmdb') return 'Saga TMDB'
      if (providers[0] === 'tvdb') return 'TheTVDB Univers'
      if (providers[0] === 'anilist') return 'AniList Relations'
    }
    return isMainlyCollection ? 'Saga & Collection' : 'Univers & Franchise'
  }, [relations, isMainlyCollection])

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
      <div className={s.cardHeader}>
        <div className={s.cardLabelWrap}>
          <span className={s.cardLabel}>
            {isMainlyCollection ? 'Saga & Collection' : 'Univers & Franchise'}
          </span>
          <span className={s.providerBadge}>{providerLabel}</span>
          <span className={s.countBadge}>({relations.length})</span>
        </div>

        <div className={s.controlsArea}>
          {relations.length > 1 && (
            <div className={s.sortToggle} role="group" aria-label="Ordre d'affichage">
              <button
                type="button"
                className={`${s.sortBtn} ${sortOrder === 'timeline' ? s.sortBtnActive : ''}`}
                onClick={() => setSortOrder('timeline')}
                title="Ordre chronologique de l'histoire"
              >
                ⏱️ Chronologie
              </button>
              <button
                type="button"
                className={`${s.sortBtn} ${sortOrder === 'release' ? s.sortBtnActive : ''}`}
                onClick={() => setSortOrder('release')}
                title="Ordre par date de sortie"
              >
                📅 Sortie
              </button>
            </div>
          )}

          <div className={s.filterTabs}>
            <button
              type="button"
              className={`${s.filterBtn} ${filter === 'all' ? s.filterBtnActive : ''}`}
              onClick={() => setFilter('all')}
            >
              Tous ({relations.length})
            </button>
            {movieCount > 0 && (
              <button
                type="button"
                className={`${s.filterBtn} ${filter === 'movies' ? s.filterBtnActive : ''}`}
                onClick={() => setFilter('movies')}
              >
                Films ({movieCount})
              </button>
            )}
            {seriesCount > 0 && (
              <button
                type="button"
                className={`${s.filterBtn} ${filter === 'series' ? s.filterBtnActive : ''}`}
                onClick={() => setFilter('series')}
              >
                Séries ({seriesCount})
              </button>
            )}
            {ovaCount > 0 && (
              <button
                type="button"
                className={`${s.filterBtn} ${filter === 'ovas' ? s.filterBtnActive : ''}`}
                onClick={() => setFilter('ovas')}
              >
                OAVs ({ovaCount})
              </button>
            )}
            {spinOffCount > 0 && (
              <button
                type="button"
                className={`${s.filterBtn} ${filter === 'spinoffs' ? s.filterBtnActive : ''}`}
                onClick={() => setFilter('spinoffs')}
              >
                Spin-offs ({spinOffCount})
              </button>
            )}
          </div>
        </div>
      </div>

      <div className={s.grid}>
        {visibleRelations.map((rel) => {
          const isWatched = rel.matched_status === 'completed'
          const isMatched = rel.matched_title_id != null

          let positionLabel = rel.format === 'MOVIE' ? 'Film' : rel.format === 'TV' ? 'Série' : rel.format
          if (rel.season_number != null) {
            positionLabel += ` · S${rel.season_number}`
          } else if (rel.relation_type === 'PREQUEL') {
            positionLabel = 'Préquelle'
          } else if (rel.relation_type === 'SEQUEL') {
            positionLabel = 'Suite'
          } else if (rel.relation_type === 'SPIN_OFF') {
            positionLabel = 'Spin-off'
          } else if (rel.relation_type === 'COLLECTION') {
            positionLabel = 'Saga'
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
                      {isWatched ? '✓ Vu' : 'À voir'}
                    </span>
                  ) : (
                    <span className={s.statusBadgeAdd}>+ Ajouter</span>
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
                      title={`Voir sur ${providerName}`}
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
          {isExpanded ? 'Voir moins' : `Voir plus (${remainingCount > 0 ? `+${remainingCount}` : ''})`}
        </button>
      )}
    </div>
  )
}
