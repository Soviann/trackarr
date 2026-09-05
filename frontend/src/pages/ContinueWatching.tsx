import { useEffect, useState, useRef } from 'preact/hooks'
import { apiFetch } from '../api'
import { ErrorBanner } from '../components/ErrorBanner'
import { PosterTile, type PosterTileItem } from '../components/PosterTile'
import type { ContinueWatchingTitle } from '../types'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import s from './PresetLibrary.module.css'

function toTile(
  t: ContinueWatchingTitle,
  onQuickMark?: (item: PosterTileItem, e: MouseEvent) => void,
  isMarking?: boolean
): PosterTileItem {
  const sublabel = t.next_episode
    ? `S${t.next_episode.season_number} E${t.next_episode.episode}`
    : (t.next_air_episode ?? '')
  return {
    id: t.id,
    type: t.type,
    is_anime: t.is_anime,
    sonarr_id: t.sonarr_id,
    radarr_id: t.radarr_id,
    cover_url: t.cover_url,
    name: t.name,
    sublabel,
    progressRatio: t.total_episodes > 0 ? t.watched_episodes / t.total_episodes : 0,
    watch_providers: t.watch_providers,
    next_episode: t.next_episode,
    onQuickMark,
    isMarking,
  }
}

export function ContinueWatching(_props: { path?: string }) {
  const [items, setItems] = useState<ContinueWatchingTitle[] | null>(null)
  useScrollRestoration('continueWatching', items !== null)
  const [error, setError] = useState<string | null>(null)
  const [markingIds, setMarkingIds] = useState<Set<number>>(new Set())
  const abortRef = useRef<AbortController | null>(null)

  const loadTitles = () => {
    abortRef.current?.abort()
    const ctrl = new AbortController()
    abortRef.current = ctrl
    apiFetch<ContinueWatchingTitle[]>('/titles/continue-watching', { signal: ctrl.signal })
      .then(data => { if (!ctrl.signal.aborted) setItems(data) })
      .catch(err => {
        if (ctrl.signal.aborted) return
        setError(err instanceof Error ? err.message : 'Failed to load')
      })
  }

  useEffect(() => {
    loadTitles()
    return () => abortRef.current?.abort()
  }, [])

  const handleQuickMark = async (item: PosterTileItem) => {
    if (!item.next_episode || markingIds.has(item.id)) return
    const currentEp = item.next_episode

    // Optimistic in-place update for immediate UI feedback
    setItems(prev => {
      if (!prev) return prev
      return prev.map(t => {
        if (t.id !== item.id) return t
        const newWatched = Math.min(t.total_episodes, t.watched_episodes + 1)
        const nextEp = t.next_episode
          ? { ...t.next_episode, episode: t.next_episode.episode + 1 }
          : null
        return {
          ...t,
          watched_episodes: newWatched,
          next_episode: nextEp,
        }
      })
    })

    setMarkingIds(prev => new Set(prev).add(item.id))
    try {
      await apiFetch(`/titles/${item.id}/episodes/${currentEp.id}`, { method: 'PATCH' })
      const data = await apiFetch<ContinueWatchingTitle[]>('/titles/continue-watching')
      setItems(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to mark episode')
      loadTitles()
    } finally {
      setMarkingIds(prev => {
        const next = new Set(prev)
        next.delete(item.id)
        return next
      })
    }
  }

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
          {items.map(t => (
            <PosterTile
              key={t.id}
              item={toTile(t, handleQuickMark, markingIds.has(t.id))}
            />
          ))}
        </div>
      )}
    </div>
  )
}
