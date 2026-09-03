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
import { useTranslation } from '../i18n'
import { ReleaseDetailSheet } from '../components/ReleaseDetailSheet'
import s from './Releases.module.css'

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'Ko', 'Mo', 'Go', 'To']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const formatted = (bytes / Math.pow(1024, i)).toFixed(i >= 3 ? 1 : 0)
  return `${formatted} ${units[i]}`
}

function formatRelativeTime(dateStr: string, t: (k: any, p?: any) => string, locale: string): string {
  if (!dateStr) return ''
  const date = new Date(dateStr)
  const now = new Date()
  const diffSec = Math.floor((now.getTime() - date.getTime()) / 1000)

  if (diffSec < 60) return t('releases.justNow')
  if (diffSec < 3600) return t('releases.minutesAgo', { count: Math.floor(diffSec / 60) })
  if (diffSec < 86400) return t('releases.hoursAgo', { count: Math.floor(diffSec / 3600) })
  const days = Math.floor(diffSec / 86400)
  if (days === 1) return t('releases.yesterday')
  if (days < 7) return t('releases.daysAgo', { count: days })
  return date.toLocaleDateString(locale === 'fr' ? 'fr-FR' : 'en-US', { day: 'numeric', month: 'short' })
}

export function Releases(_props: { path?: string }) {
  const { t, locale } = useTranslation()
  const [filterType, setFilterType] = useState<'all' | 'movie' | 'series'>('all')
  const [yearFilter, setYearFilter] = useState<string>('all')
  const [indexerFilter, setIndexerFilter] = useState<string>('all')
  const [releases, setReleases] = useState<ProwlarrRelease[]>([])
  const [selectedRelease, setSelectedRelease] = useState<ProwlarrRelease | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [addingGuid, setAddingGuid] = useState<string | null>(null)
  const [addedMap, setAddedMap] = useState<Record<string, number>>({})

  const invalidateLibrary = useTitleStore(st => st.invalidate)

  const fetchReleases = useCallback(async (forceRefresh = false) => {
    if (forceRefresh || releases.length > 0) {
      setRefreshing(true)
    } else {
      setLoading(true)
    }
    setError(null)

    try {
      const url = `/releases?type=${filterType}${forceRefresh ? '&refresh=true' : ''}`
      const data = await apiFetch<ProwlarrRelease[]>(url)
      setReleases(data ?? [])
    } catch (err) {
      setError(err instanceof Error ? err.message : t('releases.errorFetch'))
    } finally {
      setLoading(false)
      setRefreshing(false)
    }
  }, [filterType, releases.length, t])

  useEffect(() => {
    fetchReleases(false)
  }, [filterType])

  const availableIndexers = useMemo(() => {
    const names = new Set<string>()
    for (const r of releases) {
      if (r.indexer && r.indexer.trim()) {
        names.add(r.indexer.trim())
      }
    }
    return Array.from(names).sort((a, b) => a.localeCompare(b))
  }, [releases])

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
      if (indexerFilter !== 'all' && rel.indexer !== indexerFilter && String(rel.indexer_id) !== indexerFilter) {
        return false
      }
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
  }, [releases, yearFilter, indexerFilter])

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
      setError(err instanceof Error ? err.message : t('releases.errorAdd'))
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
            aria-label={t('releases.backToLibrary')}
          >
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="15 18 9 12 15 6" />
            </svg>
          </button>
          <h1 className={s.headerTitle}>{t('releases.title')}</h1>
          <button
            type="button"
            onClick={() => fetchReleases(true)}
            disabled={loading || refreshing}
            className={s.refreshBtn}
            aria-label={t('releases.refresh')}
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

        {/* Filter Row: Type Tabs + Filter Controls (Indexer + Year) */}
        <div className={s.filterRow}>
          <div className={s.tabs}>
            {(['all', 'movie', 'series'] as const).map(tab => (
              <button
                key={tab}
                type="button"
                className={clsx(s.tab, filterType === tab && s.tabActive)}
                onClick={() => setFilterType(tab)}
              >
                {tab === 'all' ? t('releases.typeAll') : tab === 'movie' ? t('releases.typeMovies') : t('releases.typeSeries')}
              </button>
            ))}
          </div>

          <div className={s.filterControls}>
            {availableIndexers.length > 1 && (
              <div className={s.selectWrap}>
                <select
                  className={clsx(s.filterSelect, indexerFilter !== 'all' && s.filterSelectActive)}
                  value={indexerFilter}
                  onChange={e => setIndexerFilter((e.target as HTMLSelectElement).value)}
                  aria-label={t('releases.filterByIndexer')}
                >
                  <option value="all">{t('releases.allIndexers')}</option>
                  {availableIndexers.map(idx => (
                    <option key={idx} value={idx}>
                      {idx}
                    </option>
                  ))}
                </select>
                {indexerFilter !== 'all' && (
                  <button
                    type="button"
                    className={s.resetFilterBtn}
                    onClick={() => setIndexerFilter('all')}
                    title={t('releases.resetIndexerFilter')}
                    aria-label={t('releases.resetIndexerFilter')}
                  >
                    ✕
                  </button>
                )}
              </div>
            )}

            <div className={s.selectWrap}>
              <select
                className={clsx(s.filterSelect, yearFilter !== 'all' && s.filterSelectActive)}
                value={yearFilter}
                onChange={e => setYearFilter((e.target as HTMLSelectElement).value)}
                aria-label={t('releases.filterByYear')}
              >
                <option value="all">{t('releases.allYears')}</option>
                <optgroup label={t('releases.recentPeriods')}>
                  <option value="gte_2025">{t('releases.recentGte2025')}</option>
                  <option value="gte_2024">≥ 2024</option>
                  <option value="gte_2020">≥ 2020</option>
                  <option value="lt_2020">{t('releases.classicsLt2020')}</option>
                </optgroup>
                {availableYears.length > 0 && (
                  <optgroup label={t('releases.exactYear')}>
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
                  className={s.resetFilterBtn}
                  onClick={() => setYearFilter('all')}
                  title={t('releases.resetYearFilter')}
                  aria-label={t('releases.resetYearFilter')}
                >
                  ✕
                </button>
              )}
            </div>
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
              const stableKey = `${rel.indexer_id || rel.indexer || 'idx'}-${rel.guid || rel.download_url || `${rel.title}-${rel.size}`}`

              return (
                <div
                  key={stableKey}
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
                      {rel.indexer && <span className={s.indexerBadge}>{rel.indexer}</span>}
                      {rel.indexer && <span className={s.metaDot}>·</span>}
                      <span>{formatBytes(rel.size)}</span>
                      <span className={s.metaDot}>·</span>
                      <span>{formatRelativeTime(rel.publish_date, t, locale)}</span>
                      <span className={s.metaDot}>·</span>
                      <span className={s.seeders}>↑ {rel.seeders} {t('releases.seeds')}</span>
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
                        {t('releases.view')}
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
                          t('releases.adding')
                        ) : (
                          <>
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                              <line x1="12" y1="5" x2="12" y2="19" />
                              <line x1="5" y1="12" x2="19" y2="12" />
                            </svg>
                            {t('releases.add')}
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
            <p>{(yearFilter !== 'all' || indexerFilter !== 'all') ? t('releases.emptyFiltered') : t('releases.emptyYear')}</p>
            <button
              type="button"
              className={s.resetBtn}
              onClick={() => {
                setYearFilter('all')
                setIndexerFilter('all')
              }}
            >
              {t('releases.resetYearFilter')}
            </button>
          </div>
        )}

        {!loading && releases.length === 0 && !error && (
          <div className={s.emptyState}>
            <p>{t('releases.emptyAll')}</p>
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
