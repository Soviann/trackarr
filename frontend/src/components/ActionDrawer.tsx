import { useState, useRef, useEffect } from 'preact/hooks'
import clsx from 'clsx'
import type { Title } from '../types'
import s from './ActionDrawer.module.css'

interface ActionDrawerProps {
  title: Title
  aniListUrl?: string | null
  onRate: () => void
  onEdit: () => void
  onRematch: () => void
  onMerge: () => void
  onRefresh: () => Promise<void>
}

export function ActionDrawer({
  title, aniListUrl,
  onRate, onEdit, onRematch, onMerge, onRefresh,
}: ActionDrawerProps) {
  const [open, setOpen] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [dragY, setDragY] = useState(0)
  const touchStartY = useRef<number | null>(null)
  const [refreshState, setRefreshState] = useState<'idle' | 'loading' | 'success' | 'error'>('idle')
  const mounted = useRef(true)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      mounted.current = false
      if (timerRef.current !== null) clearTimeout(timerRef.current)
    }
  }, [])

  const handleRefreshClick = async () => {
    if (refreshState !== 'idle') return
    setRefreshState('loading')
    try {
      await onRefresh()
      if (mounted.current) setRefreshState('success')
    } catch {
      if (mounted.current) setRefreshState('error')
    } finally {
      if (timerRef.current !== null) clearTimeout(timerRef.current)
      timerRef.current = setTimeout(() => {
        if (mounted.current) setRefreshState('idle')
      }, 2000)
    }
  }

  const handleTouchStart = (e: TouchEvent) => {
    if (!open) return
    touchStartY.current = e.touches[0].clientY
  }

  const handleTouchMove = (e: TouchEvent) => {
    if (!open || touchStartY.current === null) return
    const deltaY = e.touches[0].clientY - touchStartY.current
    if (deltaY > 0) {
      setDragY(deltaY)
    }
  }

  const handleTouchEnd = () => {
    if (!open || touchStartY.current === null) return
    if (dragY > 100) {
      setOpen(false)
      setMoreOpen(false)
    }
    setDragY(0)
    touchStartY.current = null
  }

  const toggleOpen = () => {
    const next = !open
    setOpen(next)
    if (!next) setMoreOpen(false)
  }

  const hasImdb = !!title.imdb_id
  const hasTvdb = !!title.tvdb_id
  const hasExternal = hasImdb || hasTvdb || !!aniListUrl

  return (
    <div
      className={s.container}
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
      style={dragY > 0 ? { transform: `translateY(${dragY}px)`, transition: 'none' } : undefined}
    >
      <button
        type="button"
        className={s.handle}
        onClick={toggleOpen}
        aria-expanded={open}
        aria-label={open ? 'Close actions' : 'Open actions'}
      >
        <div className={s.handleBar} />
        <span className={s.handleText}>Actions</span>
      </button>

      <div className={clsx(s.drawer, open ? s.drawerExpanded : s.drawerCollapsed)}>
        <div className={s.buttonRow}>
          <button onClick={onRate} className={s.btnPrimary}>★ Rate</button>
          <button onClick={onEdit} className={s.btnGhost}>Edit</button>
          <button
            onClick={() => setMoreOpen(!moreOpen)}
            className={clsx(s.btnGhost, moreOpen && s.btnGhostActive)}
            aria-expanded={moreOpen}
          >
            More
          </button>
        </div>

        {hasExternal && (
          <div className={s.externalRow}>
            {hasImdb && (
              <a
                href={`https://www.imdb.com/title/${title.imdb_id}/`}
                target="_blank"
                rel="noopener noreferrer"
                className={s.extImdb}
              >
                IMDb
              </a>
            )}
            {hasTvdb && (
              <a
                href={`https://thetvdb.com/dereferrer/${title.type === 'movie' ? 'movie' : 'series'}/${title.tvdb_id}`}
                target="_blank"
                rel="noopener noreferrer"
                className={s.extTvdb}
              >
                TVDB
              </a>
            )}
            {aniListUrl && (
              <a
                href={aniListUrl}
                target="_blank"
                rel="noopener noreferrer"
                className={s.extAnilist}
              >
                AniList
              </a>
            )}
          </div>
        )}

        {moreOpen && (
          <div className={s.moreSheet}>
            <button onClick={onRematch} className={s.moreBtn}>Rematch</button>
            <button onClick={onMerge} className={s.moreBtn}>Merge</button>
            <button
              onClick={handleRefreshClick}
              disabled={refreshState !== 'idle'}
              className={clsx(
                s.moreBtn,
                refreshState === 'success' && s.moreBtnSuccess,
                refreshState === 'error' && s.moreBtnError,
              )}
            >
              {refreshState === 'loading' ? '...' : refreshState === 'success' ? '✓ Done' : refreshState === 'error' ? '✗ Failed' : 'Refresh'}
            </button>
          </div>
        )}

        <div className={s.bottomPad} />
      </div>
    </div>
  )
}
