import { useState, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import type { TitleRelation } from '../types'
import { routeTo } from '../routes'
import { aniListMediaUrl, getCoverUrl } from '../utils'
import s from './FranchiseRelationsSection.module.css'

interface FranchiseRelationsSectionProps {
  relations: TitleRelation[]
}

type FilterCategory = 'all' | 'movies' | 'ovas' | 'spinoffs'

export function FranchiseRelationsSection({ relations }: FranchiseRelationsSectionProps) {
  const [filter, setFilter] = useState<FilterCategory>('all')

  const movieCount = useMemo(() => relations.filter((r) => r.format === 'MOVIE').length, [relations])
  const ovaCount = useMemo(() => relations.filter((r) => ['OVA', 'SPECIAL', 'ONA'].includes(r.format)).length, [relations])
  const spinOffCount = useMemo(() => relations.filter((r) => r.relation_type === 'SPIN_OFF').length, [relations])

  const filteredRelations = useMemo(() => {
    switch (filter) {
      case 'movies':
        return relations.filter((r) => r.format === 'MOVIE')
      case 'ovas':
        return relations.filter((r) => ['OVA', 'SPECIAL', 'ONA'].includes(r.format))
      case 'spinoffs':
        return relations.filter((r) => r.relation_type === 'SPIN_OFF')
      default:
        return relations
    }
  }, [relations, filter])

  if (!relations || relations.length === 0) {
    return null
  }

  const handleCardClick = (rel: TitleRelation) => {
    if (rel.matched_title_id != null) {
      route(routeTo.title(rel.matched_title_id))
    } else {
      route(
        `/admin/validate?q=${encodeURIComponent(
          aniListMediaUrl(rel.external_id)
        )}&name=${encodeURIComponent(rel.title)}`
      )
    }
  }

  return (
    <div className={s.section}>
      <div className={s.headerRow}>
        <div className={s.titleArea}>
          <div className={s.title}>
            <span>🌐</span>
            <span>Univers & Franchise</span>
            <span className={s.providerBadge}>AniList Relations</span>
          </div>
          <div className={s.subtitle}>
            Films, OAVs et spin-offs rattachés à la franchise ({relations.length})
          </div>
        </div>

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

      <div className={s.grid}>
        {filteredRelations.map((rel) => {
          const isWatched = rel.matched_status === 'completed'
          const isMatched = rel.matched_title_id != null

          let positionLabel = rel.format === 'MOVIE' ? 'Film' : rel.format
          if (rel.season_number != null) {
            positionLabel += ` · Après S${rel.season_number}`
          } else if (rel.relation_type === 'SPIN_OFF') {
            positionLabel = 'Spin-off'
          }

          return (
            <div
              key={rel.id || rel.external_id}
              className={s.card}
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
                    {rel.format === 'MOVIE' ? '🎬' : '🎞'}
                  </div>
                )}
              </div>

              <div className={s.cardBody}>
                <div className={s.cardTop}>
                  <span className={s.relTag}>{positionLabel}</span>
                  {isMatched ? (
                    <span className={isWatched ? s.statusBadgeWatched : s.statusBadgeUnwatched}>
                      {isWatched ? '✓ Vu' : 'À voir'}
                    </span>
                  ) : (
                    <span className={s.statusBadgeAdd}>+ Ajouter</span>
                  )}
                </div>

                <div className={s.cardTitle} title={rel.title}>
                  {rel.title}
                </div>

                <div className={s.cardMeta}>
                  {rel.year && <span>{rel.year}</span>}
                  {rel.duration && <span>· {rel.duration} min</span>}
                  {rel.score != null && <span style={{ color: '#22d3ee' }}>★ {rel.score}%</span>}
                  {!isMatched && (
                    <a
                      href={aniListMediaUrl(rel.external_id)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className={s.extLink}
                      onClick={(e) => e.stopPropagation()}
                      title="Voir sur AniList"
                    >
                      <span>AniList</span>
                      <span>↗</span>
                    </a>
                  )}
                </div>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
