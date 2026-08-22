import { useState, useEffect, useCallback, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import clsx from 'clsx'
import { apiFetch } from '../api'
import type { ProwlarrRelease, Title, TitleType } from '../types'
import { routeTo } from '../routes'
import { useTitleStore } from '../store'
import { getCoverUrl } from '../utils'
import { haptic, HAPTIC_SHORT } from '../utils/haptic'
import { StatusBadge } from '../components/StatusBadge'
import { TypeBadge } from '../components/TypeBadge'
import { ErrorBanner } from '../components/ErrorBanner'
import { PullToRefresh } from '../components/PullToRefresh'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import { ReleaseDetailSheet } from '../components/ReleaseDetailSheet'
import s from './Releases.module.css'

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const formatted = (bytes / Math.pow(1024, i)).toFixed(i >= 3 ? 1 : 0)
  return `${formatted} ${units[i]}`
}

function formatRelativeTime(dateStr: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const diffSec = Math.floor((now.getTime() - date.getTime()) / 1000)

  if (diffSec < 60) return "À l'instant"
  if (diffSec < 3600) return `Il y a ${Math.floor(diffSec / 60)} min`
  if (diffSec < 86400) return `Il y a ${Math.floor(diffSec / 3600)} h`
  const days = Math.floor(diffSec / 86400)
  if (days === 1) return 'Hier'
  if (days < 7) return `Il y a ${days} j`
  return date.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })
}

export function Releases(_props: { path?: string }) {
  const [filterType, setFilterType] = useState<'all' | 'movie' | 'series'>('all')
  const [yearFilter, setYearFilter] = useState<string>('all')
  const [releases, setReleases] = useState<ProwlarrRelease[]>([])
  const [selectedRelease, setSelectedRelease] = useState<ProwlarrRelease | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [addingGuid, setAddingGuid] = useState<string | null>(null)
  const [addedMap, setAddedMap] = useState<Record<string, number>>({})

  const invalidateLibrary = useTitleStore(st => st.invalidate)

  const fetchReleases = useCallback(async (forceRefresh = false) => {
    if (forceRefresh) setRefreshing(true)
    else setLoading(true)
    setError(null)

    try {
      const url = `/releases?type=${filterType}${forceRefresh ? '&refresh=true' : ''}`
      const data = await apiFetch<ProwlarrRelease[]>(url)
      setReleases(data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Erreur lors de la récupération des releases')
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [filterType])

  useEffect(() => {
    fetchReleases(false)
  }, [fetchReleases])

  const availableYears = useMemo(() => {
    const years = new Set<number>()
    for (const r of releases) {
      if (r.year && r.year > 0) {
        years.add(r.year)
      }
    }
    return Array.from(years).sort((a, b) => b - a)
  }, [releases])

  const filteredReleases = useMemo(() => {
    return releases.filter(rel => {
      if (yearFilter === 'all') return true
      if (yearFilter.startsWith('gte_')) {
        const minYear = parseInt(yearFilter.replace('gte_', ''), 10)
        return rel.year >= minYear
      }
      if (yearFilter.startsWith('lt_')) {
        const maxYear = parseInt(yearFilter.replace('lt_', ''), 10)
        return rel.year > 0 && rel.year < maxYear
      }
      const targetYear = parseInt(yearFilter, 10)
      return rel.year === targetYear
    })
  }, [releases, yearFilter])

  const handleAdd = async (rel: ProwlarrRelease) => {
    if (addingGuid) return
    setAddingGuid(rel.guid)
    haptic(HAPTIC_SHORT)

    try {
      const created = await apiFetch<Title>('/releases/add', {
        method: 'POST',
        body: JSON.stringify({
          tmdb_id: rel.tmdb_id,
          type: rel.type,
          title: rel.clean_title || rel.title,
          year: rel.year,
          poster_url: rel.poster_url || null,
          imdb_id: rel.imdb_id || null,
        }),
      })

      setAddedMap(prev => ({ ...prev, [rel.guid]: created.id }))
      invalidateLibrary()
    } catch (err) {
      setError(err instanceof Error ? err.message : "Erreur lors de l'ajout du titre")
    } finally {
      setAddingGuid(null)
    }
  }

  return (
    <PullToRefresh onRefresh={() => fetchReleases(true)}>
      <div className={s.page}>
        {/* Header */}
        <div className={s.header}>
          <button
            type="button"
            onClick={() => route(routeTo.home())}
            className={s.backBtn}
            aria-label="Retour à la bibliothèque"
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="15 18 9 12 15 6" />
            </svg>
          </button>
          <h1 className={s.headerTitle}>Releases</h1>
          <button
            type="button"
            onClick={() => fetchReleases(true)}
            disabled={loading || refreshing}
            className={s.refreshBtn}
            aria-label="Rafraîchir les releases"
          >
            <svg
              className={clsx(refreshing && s.spinning)}
              width="18"
              height="18"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="2"
              stroke-linecap="round"
              stroke-linejoin="round"
            >
              <polyline points="23 4 23 10 17 10" />
              <polyline points="1 20 1 14 7 14" />
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
            </svg>
          </button>
        </div>

        {/* Filter Row: Type Tabs + Year Selector */}
        <div className={s.filterRow}>
          <div className={s.tabs}>
            {(['all', 'movie', 'series'] as const).map(tab => (
              <button
                key={tab}
                type="button"
                className={clsx(s.tab, filterType === tab && s.tabActive)}
                onClick={() => setFilterType(tab)}
              >
                {tab === 'all' ? 'Tous' : tab === 'movie' ? 'Films' : 'Séries'}
              </button>
            ))}
          </div>

          <div className={s.yearFilter}>
            <select
              className={clsx(s.yearSelect, yearFilter !== 'all' && s.yearSelectActive)}
              value={yearFilter}
              onChange={e => setYearFilter((e.target as HTMLSelectElement).value)}
              aria-label="Filtrer par année"
            >
              <option value="all">Toutes années</option>
              <optgroup label="Périodes récentes">
                <option value="gte_2025">≥ 2025 (Récent)</option>
                <option value="gte_2024">≥ 2024</option>
                <option value="gte_2020">≥ 2020</option>
                <option value="lt_2020">&lt; 2020 (Classiques)</option>
              </optgroup>
              {availableYears.length > 0 && (
                <optgroup label="Année exacte">
                  {availableYears.map(yr => (
                    <option key={yr} value={String(yr)}>
                      {yr}
                    </option>
                  ))}
                </optgroup>
              )}
            </select>
            {yearFilter !== 'all' && (
              <button
                type="button"
                className={s.resetYearBtn}
                onClick={() => setYearFilter('all')}
                title="Réinitialiser l'année"
                aria-label="Réinitialiser le filtre d'année"
              >
                ✕
              </button>
            )}
          </div>
        </div>

        {/* Error */}
        {error && <ErrorBanner message={error} onRetry={() => fetchReleases(true)} />}

        {/* Skeletons */}
        {loading && (
          <div className={s.list} aria-busy="true">
            {Array.from({ length: 6 }).map((_, i) => (
              <div key={i} className={s.skeletonCard} />
            ))}
          </div>
        )}

        {/* List */}
        {!loading && filteredReleases.length > 0 && (
          <div className={s.list}>
            {filteredReleases.map(rel => {
              const existingId = rel.existing_title_id ?? addedMap[rel.guid]
              const existingStatus = rel.existing_status ?? (addedMap[rel.guid] ? 'plan_to_watch' : undefined)
              const coverUrl = getCoverUrl(rel.poster_url)

              return (
                <div
                  key={rel.guid}
                  className={s.card}
                  onClick={() => setSelectedRelease(rel)}
                >
                  {/* Poster */}
                  <div
                    className={s.coverWrap}
                    style={{ background: coverBackground(coverUrl, rel.type) }}
                  >
                    {coverUrl ? (
                      <div
                        className={s.cover}
                        style={{ backgroundImage: `url(${coverUrl})` }}
                      />
                    ) : (
                      <CoverPlaceholder type={rel.type as TitleType} iconSize="16px" />
                    )}
                  </div>

                  {/* Body */}
                  <div className={s.body}>
                    <div className={s.titleRow}>
                      <span className={s.title}>{rel.clean_title || rel.title}</span>
                      {rel.year > 0 && <span className={s.year}>{rel.year}</span>}
                      {existingStatus && <StatusBadge status={existingStatus} />}
                      <TypeBadge type={rel.type as TitleType} size="sm" />
                    </div>

                    <div className={s.rawRelease} title={rel.title}>
                      {rel.title}
                    </div>

                    <div className={s.metaRow}>
                      <span>{formatBytes(rel.size)}</span>
                      <span className={s.metaDot}>·</span>
                      <span>{formatRelativeTime(rel.publish_date)}</span>
                      <span className={s.metaDot}>·</span>
                      <span className={s.seeders}>↑ {rel.seeders} seeds</span>
                    </div>
                  </div>

                  {/* Actions */}
                  <div className={s.actions}>
                    {existingId ? (
                      <button
                        type="button"
                        className={s.viewBtn}
                        onClick={(e) => {
                          e.stopPropagation()
                          route(routeTo.title(existingId))
                        }}
                      >
                        Voir
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                          <polyline points="9 18 15 12 9 6" />
                        </svg>
                      </button>
                    ) : (
                      <button
                        type="button"
                        className={s.addBtn}
                        onClick={(e) => {
                          e.stopPropagation()
                          handleAdd(rel)
                        }}
                        disabled={addingGuid === rel.guid}
                      >
                        {addingGuid === rel.guid ? (
                          'Ajout...'
                        ) : (
                          <>
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                              <line x1="12" y1="5" x2="12" y2="19" />
                              <line x1="5" y1="12" x2="19" y2="12" />
                            </svg>
                            Ajouter
                          </>
                        )}
                      </button>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        )}

        {!loading && releases.length > 0 && filteredReleases.length === 0 && !error && (
          <div className={s.emptyState}>
            <p>Aucune release trouvée pour l'année sélectionnée.</p>
            <button
              type="button"
              className={s.resetBtn}
              onClick={() => setYearFilter('all')}
            >
              Réinitialiser le filtre d'année
            </button>
          </div>
        )}

        {!loading && releases.length === 0 && !error && (
          <div className={s.emptyState}>
            <p>Aucune release disponible pour le moment.</p>
          </div>
        )}

        {/* Release Detail BottomSheet */}
        <ReleaseDetailSheet
          release={selectedRelease}
          onClose={() => setSelectedRelease(null)}
          onAdd={handleAdd}
          adding={addingGuid === selectedRelease?.guid}
          existingTitleId={selectedRelease ? (selectedRelease.existing_title_id ?? addedMap[selectedRelease.guid]) : undefined}
          existingStatus={selectedRelease ? (selectedRelease.existing_status ?? (addedMap[selectedRelease.guid] ? 'plan_to_watch' : undefined)) : undefined}
        />
      </div>
    </PullToRefresh>
  )
}
