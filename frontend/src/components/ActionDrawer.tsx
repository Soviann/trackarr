import { useState, useRef, useEffect, useCallback } from 'preact/hooks'
import clsx from 'clsx'
import { useSwipeDownToClose } from '../hooks/useSwipeDownToClose'
import type { Title } from '../types'
import s from './ActionDrawer.module.css'

interface ActionDrawerProps {
  title: Title
  onRate: () => void
  onEdit: () => void
  onRematch: () => void
  onMerge: () => void
  onRefresh: () => Promise<void>
  onDelete: () => void
  onOpenChange?: (open: boolean) => void
}

export function ActionDrawer({
  title: _title,
  onRate, onEdit, onRematch, onMerge, onRefresh, onDelete, onOpenChange,
}: ActionDrawerProps) {
  const [open, setOpen] = useState(false)
  const [moreOpen, setMoreOpen] = useState(false)
  const [refreshState, setRefreshState] = useState<'idle' | 'loading' | 'success' | 'error'>('idle')
  const mounted = useRef(true)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const handleClose = useCallback(() => {
    setOpen(false)
    setMoreOpen(false)
  }, [])

  const { ref: containerRef, style: swipeStyle } = useSwipeDownToClose({
    open,
    onClose: handleClose,
  })

  useEffect(() => {
    return () => {
      mounted.current = false
      if (timerRef.current !== null) clearTimeout(timerRef.current)
    }
  }, [])

  // Surface open state so the page can disable pull-to-refresh while the
  // drawer's swipe-down-to-close gesture is active (they would otherwise fight).
  useEffect(() => {
    onOpenChange?.(open)
  }, [open])

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

  const toggleOpen = () => {
    const next = !open
    setOpen(next)
    if (!next) setMoreOpen(false)
  }

  return (
    <div
      ref={containerRef}
      className={s.container}
      style={swipeStyle}
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
            <button onClick={onDelete} className={clsx(s.moreBtn, s.moreBtnDanger)}>Delete</button>
          </div>
        )}

        <div className={s.bottomPad} />
      </div>
    </div>
  )
}
