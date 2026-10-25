import { useEffect, useState, useRef, useMemo } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import { routeTo } from '../routes'
import { ErrorBanner } from '../components/ErrorBanner'
import { PosterTile, type PosterTileItem } from '../components/PosterTile'
import type { UpcomingTitle } from '../types'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import { useTranslation, type Locale, type TranslationKey } from '../i18n'
import s from './ComingUp.module.css'

function airDateBadge(
  dateStr: string,
  locale: Locale,
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
): { label: string; dateFormatted: string; variant: 'amber' | 'teal' | 'muted' } {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const air = new Date(`${dateStr}T00:00:00`)
  air.setHours(0, 0, 0, 0)
  const diffDays = Math.round((air.getTime() - today.getTime()) / 86_400_000)
  const dateFormatted = air.toLocaleDateString(locale === 'fr' ? 'fr-FR' : 'en-US', { day: 'numeric', month: 'short' })

  if (diffDays === 0) {
    return { label: t('calendar.today'), dateFormatted, variant: 'amber' }
  }
  if (diffDays > 0 && diffDays <= 6) {
    return {
      label: air.toLocaleDateString(locale === 'fr' ? 'fr-FR' : 'en-US', { weekday: 'short' }),
      dateFormatted,
      variant: 'teal',
    }
  }
  if (diffDays > 6) {
    return {
      label: t('calendar.inDays', { days: diffDays }),
      dateFormatted,
      variant: 'muted',
    }
  }
  return { label: dateFormatted, dateFormatted, variant: 'muted' }
}

function toTile(
  item: UpcomingTitle,
  locale: Locale,
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
): PosterTileItem {
  const { label, dateFormatted, variant } = airDateBadge(item.next_air_date, locale, t)
  const epTag = item.next_air_episode ? item.next_air_episode.trim() : ''

  return {
    id: item.id,
    type: item.type,
    is_anime: item.is_anime,
    sonarr_id: item.sonarr_id,
    radarr_id: item.radarr_id,
    cover_url: item.cover_url,
    name: item.name,
    sublabel: epTag ? `${epTag} · ${dateFormatted}` : dateFormatted,
    badgeLabel: label,
    sublabelVariant: variant,
    watch_providers: item.watch_providers,
  }
}

export function ComingUp(_props: { path?: string }) {
  const { t, locale } = useTranslation()
  const [typeFilter, setTypeFilter] = useState<'all' | 'movie' | 'series' | 'anime'>('all')
  const [titles, setTitles] = useState<UpcomingTitle[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  useScrollRestoration('comingUp', titles !== null)

  const loadTitles = () => {
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl
    setError(null)

    apiFetch<UpcomingTitle[]>('/titles/upcoming', { signal: ctrl.signal })
      .then((data) => {
        if (!ctrl.signal.aborted) setTitles(data ?? [])
      })
      .catch((err) => {
        if (ctrl.signal.aborted) return
        setError(err instanceof Error ? err.message : t('upcoming.loadError'))
      })
  }

  useEffect(() => {
    loadTitles()
    return () => abortRef.current?.abort()
  }, [])

  const filteredTitles = useMemo(() => {
    if (!titles) return []
    if (typeFilter === 'all') return titles
    if (typeFilter === 'anime') return titles.filter((item) => item.is_anime)
    if (typeFilter === 'movie') return titles.filter((item) => item.type === 'movie')
    if (typeFilter === 'series') return titles.filter((item) => item.type === 'series' && !item.is_anime)
    return titles
  }, [titles, typeFilter])

  return (
    <div className={s.page}>
      {/* Header */}
      <div className={s.header}>
        <div className={s.headerLeft}>
          <button onClick={() => history.back()} aria-label={t('common.back')} className={s.backBtn}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" />
              <polyline points="12 19 5 12 12 5" />
            </svg>
          </button>
          <div className={s.headerText}>
            <span className={s.label}>// {t('upcoming.title')}</span>
            {titles && (
              <span className={s.count}>
                {t('upcoming.airingSoon', {
                  count: filteredTitles.length,
                })}
              </span>
            )}
          </div>
        </div>

        <button
          onClick={() => route(routeTo.calendar())}
          className={s.calendarBtn}
          title={t('upcoming.openCalendar')}
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
            <line x1="16" y1="2" x2="16" y2="6" />
            <line x1="8" y1="2" x2="8" y2="6" />
            <line x1="3" y1="10" x2="21" y2="10" />
          </svg>
          <span>{t('upcoming.openCalendar')}</span>
        </button>
      </div>

      {/* Control Bar: Category filters */}
      <div className={s.controlBar}>
        <div className={s.typeFilters} role="group" aria-label={t('search.filters')}>
          <button
            className={`${s.typeChip}${typeFilter === 'all' ? ` ${s.typeChipActive}` : ''}`}
            onClick={() => setTypeFilter('all')}
          >
            {t('franchise.all')}
          </button>
          <button
            className={`${s.typeChip}${typeFilter === 'series' ? ` ${s.typeChipActive}` : ''}`}
            onClick={() => setTypeFilter('series')}
          >
            📺 {t('franchise.series')}
          </button>
          <button
            className={`${s.typeChip}${typeFilter === 'anime' ? ` ${s.typeChipActive}` : ''}`}
            onClick={() => setTypeFilter('anime')}
          >
            ⛩️ Anime
          </button>
          <button
            className={`${s.typeChip}${typeFilter === 'movie' ? ` ${s.typeChipActive}` : ''}`}
            onClick={() => setTypeFilter('movie')}
          >
            🎬 {t('franchise.movies')}
          </button>
        </div>
      </div>

      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

      {/* Main Content Area */}
      <div className={s.contentArea}>
        {titles === null && (
          <div className={s.grid} aria-busy="true" aria-label={t('common.loading')}>
            {Array.from({ length: 9 }).map((_, i) => (
              <div key={i} className={s.skeletonTile} aria-hidden="true" />
            ))}
          </div>
        )}

        {titles && filteredTitles.length === 0 && (
          <div className={s.empty}>
            {titles.length === 0 ? t('upcoming.empty') : t('upcoming.emptyFiltered')}
          </div>
        )}

        {titles && filteredTitles.length > 0 && (
          <div className={s.grid}>
            {filteredTitles.map((item) => (
              <PosterTile key={item.id} item={toTile(item, locale, t)} />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
