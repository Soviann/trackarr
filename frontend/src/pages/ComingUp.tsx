import { useEffect, useState, useRef } from 'preact/hooks'
import { apiFetch } from '../api'
import { ErrorBanner } from '../components/ErrorBanner'
import { PosterTile, type PosterTileItem } from '../components/PosterTile'
import type { UpcomingTitle } from '../types'
import { isOnPrime } from '../utils/providers'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import s from './PresetLibrary.module.css'

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

function toTile(t: UpcomingTitle): PosterTileItem {
  const { label, variant } = airDateBadge(t.next_air_date)
  return {
    id: t.id,
    type: t.type,
    is_anime: t.is_anime,
    sonarr_id: t.sonarr_id,
    radarr_id: t.radarr_id,
    cover_url: t.cover_url,
    name: t.name,
    sublabel: label,
    sublabelVariant: variant,
    onPrime: isOnPrime(t.watch_providers),
  }
}

export function ComingUp(_props: { path?: string }) {
  const [items, setItems] = useState<UpcomingTitle[] | null>(null)
  useScrollRestoration('comingUp', items !== null)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl
    apiFetch<UpcomingTitle[]>('/titles/upcoming', { signal: ctrl.signal })
      .then(data => { if (!ctrl.signal.aborted) setItems(data) })
      .catch(err => {
        if (ctrl.signal.aborted) return
        setError(err instanceof Error ? err.message : 'Failed to load')
      })
    return () => ctrl.abort()
  }, [])

  return (
    <div className={s.page}>
      <div className={s.header}>
        <button onClick={() => history.back()} aria-label="Back" className={s.backBtn}>
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
        <div className={s.headerText}>
          <span className={s.label}>// Coming Up</span>
          {items && (
            <span className={s.count}>
              {items.length} title{items.length === 1 ? '' : 's'} airing soon
            </span>
          )}
        </div>
      </div>

      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

      {items === null && (
        <div className={s.grid} aria-busy="true" aria-label="Loading coming up">
          {Array.from({ length: 9 }).map((_, i) => (
            <div key={i} className={s.skeletonTile} aria-hidden="true" />
          ))}
        </div>
      )}

      {items && items.length === 0 && (
        <div className={s.empty}>Nothing airing soon.</div>
      )}

      {items && items.length > 0 && (
        <div className={s.grid}>
          {items.map(t => <PosterTile key={t.id} item={toTile(t)} />)}
        </div>
      )}
    </div>
  )
}
