import { useEffect, useState, useRef, useMemo } from 'preact/hooks'
import { apiFetch } from '../api'
import { ErrorBanner } from '../components/ErrorBanner'
import { PosterTile, type PosterTileItem } from '../components/PosterTile'
import { CalendarMonthGrid } from '../components/CalendarMonthGrid'
import { CalendarWeekTimeline } from '../components/CalendarWeekTimeline'
import { CalendarIcalModal } from '../components/CalendarIcalModal'
import type { CalendarEvent, CalendarViewMode } from '../types'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import s from './ComingUp.module.css'

function airDateBadge(dateStr: string): { label: string; variant: 'amber' | 'teal' | 'muted' } {
  const today = new Date()
  today.setHours(0, 0, 0, 0)
  const air = new Date(dateStr)
  air.setHours(0, 0, 0, 0)
  const diffDays = Math.round((air.getTime() - today.getTime()) / 86_400_000)
  if (diffDays === 0) return { label: 'Today', variant: 'amber' }
  if (diffDays <= 6) return { label: air.toLocaleDateString('en-US', { weekday: 'short' }), variant: 'teal' }
  return { label: `in ${diffDays}d`, variant: 'muted' }
}

function toTile(ev: CalendarEvent): PosterTileItem {
  const { label, variant } = airDateBadge(ev.air_date)
  const epTag =
    ev.season_number != null && ev.episode_number != null
      ? `S${String(ev.season_number).padStart(2, '0')}E${String(ev.episode_number).padStart(2, '0')}`
      : ev.next_air_episode || ''

  return {
    id: ev.title_id,
    type: ev.type,
    is_anime: ev.is_anime,
    sonarr_id: ev.sonarr_id,
    radarr_id: ev.radarr_id,
    cover_url: ev.cover_url,
    name: ev.title_name,
    sublabel: epTag ? `${epTag} · ${label}` : label,
    sublabelVariant: variant,
    watch_providers: ev.watch_providers,
  }
}

export function ComingUp(_props: { path?: string }) {
  const [viewMode, setViewMode] = useState<CalendarViewMode>(() => {
    const saved = localStorage.getItem('trackarr_calendar_view')
    if (saved === 'week' || saved === 'list' || saved === 'month') {
      return saved
    }
    return 'month'
  })

  const [typeFilter, setTypeFilter] = useState<'all' | 'movie' | 'series' | 'anime'>('all')
  const [events, setEvents] = useState<CalendarEvent[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isIcalOpen, setIsIcalOpen] = useState(false)
  const abortRef = useRef<AbortController | null>(null)

  useScrollRestoration('comingUp', events !== null && viewMode === 'list')

  const handleSetViewMode = (mode: CalendarViewMode) => {
    setViewMode(mode)
    localStorage.setItem('trackarr_calendar_view', mode)
  }

  useEffect(() => {
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl

    // Fetch calendar events
    apiFetch<CalendarEvent[]>('/calendar/events', { signal: ctrl.signal })
      .then((data) => {
        if (!ctrl.signal.aborted) setEvents(data)
      })
      .catch((err) => {
        if (ctrl.signal.aborted) return
        setError(err instanceof Error ? err.message : 'Impossible de charger le calendrier')
      })
    return () => ctrl.abort()
  }, [])

  const filteredEvents = useMemo(() => {
    if (!events) return []
    if (typeFilter === 'all') return events
    if (typeFilter === 'anime') return events.filter((e) => e.is_anime)
    if (typeFilter === 'movie') return events.filter((e) => e.type === 'movie')
    if (typeFilter === 'series') return events.filter((e) => e.type === 'series' && !e.is_anime)
    return events
  }, [events, typeFilter])

  return (
    <div className={s.page}>
      {/* Header */}
      <div className={s.header}>
        <div className={s.headerLeft}>
          <button onClick={() => history.back()} aria-label="Retour" className={s.backBtn}>
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" />
              <polyline points="12 19 5 12 12 5" />
            </svg>
          </button>
          <div className={s.headerText}>
            <span className={s.label}>// Calendrier</span>
            {events && (
              <span className={s.count}>
                {filteredEvents.length} sortie{filteredEvents.length === 1 ? '' : 's'} à venir
              </span>
            )}
          </div>
        </div>

        <button
          onClick={() => setIsIcalOpen(true)}
          className={s.icalBtn}
          title="S'abonner au flux iCal"
        >
          <span>📅</span>
          <span>Flux iCal</span>
        </button>
      </div>

      {/* Control Bar: View selector + Category filters */}
      <div className={s.controlBar}>
        <div className={s.viewSelector} role="tablist" aria-label="Mode de vue">
          <button
            role="tab"
            aria-selected={viewMode === 'month'}
            className={`${s.viewTab}${viewMode === 'month' ? ` ${s.viewTabActive}` : ''}`}
            onClick={() => handleSetViewMode('month')}
          >
            📅 Mois
          </button>
          <button
            role="tab"
            aria-selected={viewMode === 'week'}
            className={`${s.viewTab}${viewMode === 'week' ? ` ${s.viewTabActive}` : ''}`}
            onClick={() => handleSetViewMode('week')}
          >
            📆 Semaine
          </button>
          <button
            role="tab"
            aria-selected={viewMode === 'list'}
            className={`${s.viewTab}${viewMode === 'list' ? ` ${s.viewTabActive}` : ''}`}
            onClick={() => handleSetViewMode('list')}
          >
            📋 Liste
          </button>
        </div>

        <div className={s.typeFilters} role="group" aria-label="Filtres par type">
          <button
            className={`${s.typeChip}${typeFilter === 'all' ? ` ${s.typeChipActive}` : ''}`}
            onClick={() => setTypeFilter('all')}
          >
            Tous
          </button>
          <button
            className={`${s.typeChip}${typeFilter === 'movie' ? ` ${s.typeChipActive}` : ''}`}
            onClick={() => setTypeFilter('movie')}
          >
            🎬 Films
          </button>
          <button
            className={`${s.typeChip}${typeFilter === 'series' ? ` ${s.typeChipActive}` : ''}`}
            onClick={() => setTypeFilter('series')}
          >
            📺 Séries
          </button>
          <button
            className={`${s.typeChip}${typeFilter === 'anime' ? ` ${s.typeChipActive}` : ''}`}
            onClick={() => setTypeFilter('anime')}
          >
            ⛩️ Anime
          </button>
        </div>
      </div>

      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

      {/* Main Content Area */}
      <div className={s.contentArea}>
        {events === null && (
          <div className={s.grid} aria-busy="true" aria-label="Chargement du calendrier">
            {Array.from({ length: 9 }).map((_, i) => (
              <div key={i} className={s.skeletonTile} aria-hidden="true" />
            ))}
          </div>
        )}

        {events && filteredEvents.length === 0 && (
          <div className={s.empty}>Aucune sortie prévue pour les critères sélectionnés.</div>
        )}

        {events && filteredEvents.length > 0 && (
          <>
            {viewMode === 'month' && <CalendarMonthGrid events={filteredEvents} />}
            {viewMode === 'week' && <CalendarWeekTimeline events={filteredEvents} />}
            {viewMode === 'list' && (
              <div className={s.grid}>
                {filteredEvents.map((ev) => (
                  <PosterTile key={ev.id} item={toTile(ev)} />
                ))}
              </div>
            )}
          </>
        )}
      </div>

      {/* iCal Subscription Modal */}
      <CalendarIcalModal
        isOpen={isIcalOpen}
        onClose={() => setIsIcalOpen(false)}
      />
    </div>
  )
}
