import { useState, useRef } from 'preact/hooks'
import clsx from 'clsx'
import type { Episode } from '../types'
import { haptic, HAPTIC_SHORT } from '../utils/haptic'
import { useTranslation } from '../i18n'
import s from './EpisodeRow.module.css'

interface EpisodeRowProps {
  episode: Episode
  onToggle: (episodeId: number) => void | Promise<void>
}

const SWIPE_THRESHOLD = 64

export function EpisodeRow({ episode, onToggle }: EpisodeRowProps) {
  const { t } = useTranslation()
  const [toggling, setToggling] = useState(false)
  const [offset, setOffset] = useState(0)
  const [isDragging, setIsDragging] = useState(false)
  const [thresholdPassed, setThresholdPassed] = useState(false)

  const activePointerRef = useRef<number | null>(null)
  const startXRef = useRef(0)
  const startYRef = useRef(0)
  const directionLockedRef = useRef<'horizontal' | 'vertical' | null>(null)
  const hapticFiredRef = useRef(false)

  const handleToggle = async (e?: Event) => {
    e?.stopPropagation()
    if (toggling) return
    setToggling(true)
    try {
      await onToggle(episode.id)
    } finally {
      setToggling(false)
    }
  }

  const handlePointerDown = (e: PointerEvent) => {
    if (e.pointerType === 'mouse' || toggling) return
    activePointerRef.current = e.pointerId
    startXRef.current = e.clientX
    startYRef.current = e.clientY
    directionLockedRef.current = null
    hapticFiredRef.current = false
    setIsDragging(true)
  }

  const handlePointerMove = (e: PointerEvent) => {
    if (activePointerRef.current !== e.pointerId) return

    const dx = e.clientX - startXRef.current
    const dy = e.clientY - startYRef.current

    if (directionLockedRef.current === null) {
      if (Math.abs(dx) < 10 && Math.abs(dy) < 10) return
      directionLockedRef.current = Math.abs(dx) >= Math.abs(dy) ? 'horizontal' : 'vertical'
      if (directionLockedRef.current === 'horizontal') {
        const el = e.currentTarget as HTMLElement | null
        el?.setPointerCapture?.(e.pointerId)
      }
    }

    if (directionLockedRef.current === 'vertical') return

    const absDx = Math.abs(dx)
    const passed = absDx >= SWIPE_THRESHOLD
    setThresholdPassed(passed)

    if (passed && !hapticFiredRef.current) {
      haptic(HAPTIC_SHORT)
      hapticFiredRef.current = true
    } else if (!passed && hapticFiredRef.current) {
      hapticFiredRef.current = false
    }

    // Visual damping beyond threshold
    const damped = passed
      ? Math.sign(dx) * (SWIPE_THRESHOLD + (absDx - SWIPE_THRESHOLD) * 0.35)
      : dx

    setOffset(damped)
  }

  const handlePointerUp = (e: PointerEvent) => {
    if (activePointerRef.current !== e.pointerId) return
    activePointerRef.current = null
    setIsDragging(false)

    if (hapticFiredRef.current) {
      void handleToggle()
    }

    setOffset(0)
    setThresholdPassed(false)
    directionLockedRef.current = null
  }

  const handlePointerCancel = (e: PointerEvent) => {
    if (activePointerRef.current !== e.pointerId) return
    activePointerRef.current = null
    setIsDragging(false)
    setOffset(0)
    setThresholdPassed(false)
    directionLockedRef.current = null
  }

  const isLeftSwipe = offset > 0
  const isRightSwipe = offset < 0
  const showBackdrop = offset !== 0

  return (
    <div
      className={s.swipeContainer}
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerCancel}
    >
      {/* Background action backdrop revealed on swipe */}
      {showBackdrop && (
        <div
          className={clsx(
            s.swipeBackdrop,
            isLeftSwipe ? s.swipeBackdropLeft : s.swipeBackdropRight,
            episode.watched
              ? (thresholdPassed ? s.swipeBackdropUnmarkActive : s.swipeBackdropUnmark)
              : (thresholdPassed ? s.swipeBackdropMarkActive : s.swipeBackdropMark)
          )}
          aria-hidden="true"
        >
          {episode.watched ? (
            <>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
              <span>{t('details.swipeToUnmark')}</span>
            </>
          ) : (
            <>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="20 6 9 17 4 12" />
              </svg>
              <span>{t('details.swipeToMark')}</span>
            </>
          )}
        </div>
      )}

      {/* Foreground row */}
      <div
        className={clsx(
          s.row,
          episode.watched && s.watched,
          isDragging ? s.rowDragging : s.rowResting
        )}
        style={{ transform: offset ? `translateX(${offset}px)` : undefined }}
      >
        <div className={s.info}>
          <span className={s.episodeNumber}>E{episode.episode}</span>
          {episode.name && (
            <span className={s.episodeName}>— {episode.name}</span>
          )}
          {episode.air_date && (
            <span className={s.airDate}>{episode.air_date}</span>
          )}
        </div>

        <button
          type="button"
          onClick={handleToggle}
          className={s.toggle}
          aria-label={episode.watched ? t('details.swipeToUnmark') : t('details.swipeToMark')}
        >
          {episode.watched ? (
            <svg width="18" height="18" viewBox="0 0 24 24" fill="var(--accent)" stroke="none">
              <path d="M20 6L9 17l-5-5 1.41-1.41L9 14.17 18.59 4.58z" />
            </svg>
          ) : (
            <div className={s.checkbox} />
          )}
        </button>
      </div>
    </div>
  )
}
