import { useEffect, useState, useRef } from 'preact/hooks'
import { apiFetch } from '../api'
import { ErrorBanner } from '../components/ErrorBanner'
import { PosterTile, type PosterTileItem } from '../components/PosterTile'
import type { ContinueWatchingTitle } from '../types'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import { useTranslation } from '../i18n'
import { useUndo } from '../context/UndoContext'
import s from './PresetLibrary.module.css'

function toTile(
  tItem: ContinueWatchingTitle,
  t: ReturnType<typeof useTranslation>['t'],
  onQuickMark?: (item: PosterTileItem, e: MouseEvent) => void,
  isMarking?: boolean
): PosterTileItem {
  let sublabel = tItem.next_episode
    ? `S${tItem.next_episode.season_number} E${tItem.next_episode.episode}`
    : (tItem.next_air_episode ?? '')
  if (tItem.is_saga_next_up) {
    if (tItem.next_episode) {
      sublabel = `S${tItem.next_episode.season_number} E${tItem.next_episode.episode} • ${t('continueWatching.saga')}`
    } else {
      sublabel = t('continueWatching.sagaNextUp')
    }
  }
  return {
    id: tItem.id,
    type: tItem.type,
    is_anime: tItem.is_anime,
    sonarr_id: tItem.sonarr_id,
    radarr_id: tItem.radarr_id,
    cover_url: tItem.cover_url,
    name: tItem.name,
    sublabel,
    progressRatio: tItem.total_episodes > 0 ? tItem.watched_episodes / tItem.total_episodes : 0,
    watch_providers: tItem.watch_providers,
    next_episode: tItem.next_episode,
    is_saga_next_up: tItem.is_saga_next_up,
    saga_source_title: tItem.saga_source_title,
    onQuickMark,
    isMarking,
  }
}

export function ContinueWatching(_props: { path?: string }) {
  const { t } = useTranslation()
  const { showUndo } = useUndo()
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
    if (markingIds.has(item.id)) return
    if (!item.next_episode && item.type !== 'movie') return

    if (item.type === 'movie') {
      setMarkingIds(prev => new Set(prev).add(item.id))
      try {
        await apiFetch(`/titles/${item.id}`, { method: 'PATCH', body: JSON.stringify({ status: 'completed' }) })
        const data = await apiFetch<ContinueWatchingTitle[]>('/titles/continue-watching')
        setItems(data)
        showUndo({
          message: t('undo.movieWatched', { title: item.name }),
          onUndo: async () => {
            await apiFetch(`/titles/${item.id}`, { method: 'PATCH', body: JSON.stringify({ status: 'plan_to_watch' }) })
            loadTitles()
          },
        })
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to mark movie')
        loadTitles()
      } finally {
        setMarkingIds(prev => {
          const next = new Set(prev)
          next.delete(item.id)
          return next
        })
      }
      return
    }

    const currentEp = item.next_episode!

    // Optimistic in-place update for immediate UI feedback
    setItems(prev => {
      if (!prev) return prev
      return prev.map(tItem => {
        if (tItem.id !== item.id) return tItem
        const newWatched = Math.min(tItem.total_episodes, tItem.watched_episodes + 1)
        const nextEp = tItem.next_episode
          ? { ...tItem.next_episode, episode: tItem.next_episode.episode + 1 }
          : null
        return {
          ...tItem,
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
      showUndo({
        message: t('undo.quickMarked', {
          title: item.name,
          ep: `S${currentEp.season_number}E${currentEp.episode}`,
        }),
        onUndo: async () => {
          await apiFetch(`/titles/${item.id}/episodes/${currentEp.id}`, { method: 'PATCH' })
          loadTitles()
        },
      })
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
          <span className={s.label}>{t('continueWatching.title')}</span>
          {items && (
            <span className={s.count}>
              {t('continueWatching.inProgressCount', { count: items.length })}
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
        <div className={s.empty}>{t('continueWatching.empty')}</div>
      )}

      {items && items.length > 0 && (
        <div className={s.grid}>
          {items.map(cwItem => (
            <PosterTile
              key={cwItem.id}
              item={toTile(cwItem, t, handleQuickMark, markingIds.has(cwItem.id))}
            />
          ))}
        </div>
      )}
    </div>
  )
}
