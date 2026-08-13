import { useEffect, useState, useRef } from 'preact/hooks'
import { apiFetch } from '../api'
import { ErrorBanner } from '../components/ErrorBanner'
import { PosterTile, type PosterTileItem } from '../components/PosterTile'
import type { ContinueWatchingTitle } from '../types'
import { isOnPrime } from '../utils/providers'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import s from './PresetLibrary.module.css'

function toTile(t: ContinueWatchingTitle): PosterTileItem {
  return {
    id: t.id,
    type: t.type,
    cover_url: t.cover_url,
    name: t.name,
    sublabel: t.next_air_episode ?? '',
    progressRatio: t.total_episodes > 0 ? t.watched_episodes / t.total_episodes : 0,
    onPrime: isOnPrime(t.watch_providers),
  }
}

export function ContinueWatching(_props: { path?: string }) {
  const [items, setItems] = useState<ContinueWatchingTitle[] | null>(null)
  useScrollRestoration('continueWatching', items !== null)
  const [error, setError] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl
    apiFetch<ContinueWatchingTitle[]>('/titles/continue-watching', { signal: ctrl.signal })
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
          <span className={s.label}>// Continue Watching</span>
          {items && (
            <span className={s.count}>
              {items.length} in progress
            </span>
          )}
        </div>
      </div>

      {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

      {items === null && (
        <div className={s.grid} aria-busy="true" aria-label="Loading continue watching">
          {Array.from({ length: 9 }).map((_, i) => (
            <div key={i} className={s.skeletonTile} aria-hidden="true" />
          ))}
        </div>
      )}

      {items && items.length === 0 && (
        <div className={s.empty}>Nothing in progress.</div>
      )}

      {items && items.length > 0 && (
        <div className={s.grid}>
          {items.map(t => <PosterTile key={t.id} item={toTile(t)} />)}
        </div>
      )}
    </div>
  )
}
