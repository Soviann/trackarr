import { useState, useRef, useEffect, useCallback } from 'preact/hooks'
import clsx from 'clsx'
import { useSwipeDownToClose } from '../hooks/useSwipeDownToClose'
import { useTranslation } from '../i18n'
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
  const { t } = useTranslation()
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
        aria-label={open ? t('common.closeActions') : t('common.openActions')}
      >
        <div className={s.handleBar} />
        <span className={s.handleText}>{t('common.actions')}</span>
      </button>

      <div className={clsx(s.drawer, open ? s.drawerExpanded : s.drawerCollapsed)}>
        <div className={s.buttonRow}>
          <button onClick={onRate} className={s.btnPrimary}>★ {t('common.rate')}</button>
          <button onClick={onEdit} className={s.btnGhost}>{t('common.edit')}</button>
          <button
            onClick={() => setMoreOpen(!moreOpen)}
            className={clsx(s.btnGhost, moreOpen && s.btnGhostActive)}
            aria-expanded={moreOpen}
          >
            {t('common.more')}
          </button>
        </div>

        {moreOpen && (
          <div className={s.moreSheet}>
            <button onClick={onRematch} className={s.moreBtn}>{t('common.rematch')}</button>
            <button onClick={onMerge} className={s.moreBtn}>{t('common.merge')}</button>
            <button
              onClick={handleRefreshClick}
              disabled={refreshState !== 'idle'}
              className={clsx(
                s.moreBtn,
                refreshState === 'success' && s.moreBtnSuccess,
                refreshState === 'error' && s.moreBtnError,
              )}
            >
              {refreshState === 'loading' ? '...' : refreshState === 'success' ? t('common.refreshDone') : refreshState === 'error' ? t('common.refreshFailed') : t('common.refresh')}
            </button>
            <button onClick={onDelete} className={clsx(s.moreBtn, s.moreBtnDanger)}>{t('common.delete')}</button>
          </div>
        )}

        <div className={s.bottomPad} />
      </div>
    </div>
  )
}
