import { useEffect, useState, useCallback, useRef } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import { useApi } from '../hooks/useApi'
import { useScrollRestoration } from '../hooks/useScrollRestoration'
import { haptic, HAPTIC_SHORT } from '../utils/haptic'
import { formatWatchtimeShort, getCoverUrl } from '../utils'
import type { Title, ContinueWatchingTitle, UpcomingTitle, StatsResponse } from '../types'
import { colors } from '../theme'
import { useTitleStore } from '../store'
import { routeTo } from '../routes'
import { TitleCard } from '../components/TitleCard'
import { PosterCard } from '../components/PosterCard'
import { ErrorBanner } from '../components/ErrorBanner'
import { SectionCards, type CardItemProps } from '../components/SectionCards'
import { BottomSheet } from '../components/BottomSheet'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import { PullToRefresh } from '../components/PullToRefresh'
import s from './Library.module.css'

function TitleList({ titles, onUpdate }: { titles: Title[]; onUpdate: () => void }) {
  if (titles.length === 0) return null
  return (
    <div className={s.titleList}>
      {titles.map((t) => <TitleCard key={t.id} title={t} onUpdate={onUpdate} />)}
    </div>
  )
}

function PosterGrid({ titles }: { titles: Title[] }) {
  if (titles.length === 0) return null
  return (
    <div className={s.posterGrid}>
      {titles.map((t) => <PosterCard key={t.id} title={t} />)}
    </div>
  )
}

interface MatchReviewBannerProps {
  count: number
  pendingCount: number
  unconfirmedCount: number
}

function MatchReviewBanner({ count, pendingCount, unconfirmedCount }: MatchReviewBannerProps) {
  if (count === 0) return null
  return (
    <button type="button" onClick={() => route(routeTo.matchReview())} className={s.bannerWrapper}>
      <div className={s.banner}>
        <div className={s.bannerBadge}>
          <span className={s.bannerBadgeText}>{count}</span>
        </div>
        <div className={s.bannerBody}>
          <span className={s.bannerTitle}>
            {count} title{count > 1 ? 's' : ''} need{count === 1 ? 's' : ''} review
          </span>
          <span className={s.bannerSub}>
            {pendingCount} pending · {unconfirmedCount} unconfirmed
          </span>
        </div>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke={colors.inkDim} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="9 18 15 12 9 6" />
        </svg>
      </div>
    </button>
  )
}

function LoadMoreButton({ onClick, loading }: { onClick: () => void; loading: boolean }) {
  return (
    <div className={s.loadMoreWrapper}>
      <button onClick={onClick} disabled={loading} className={s.loadMoreBtn}>
        {loading ? 'Loading...' : 'Load more'}
      </button>
    </div>
  )
}

export function Library(_props: { path?: string }) {
  const titles = useTitleStore(s => s.titles)
  const total = useTitleStore(s => s.total)
  const hasMore = useTitleStore(s => s.hasMore)
  const counts = useTitleStore(s => s.counts)
  const filter = useTitleStore(s => s.filter)
  const loading = useTitleStore(s => s.loading)
  useScrollRestoration('library', !loading)
  const loadingMore = useTitleStore(s => s.loadingMore)
  const error = useTitleStore(s => s.error)
  const fetchTitles = useTitleStore(s => s.fetchTitles)
  const loadMore = useTitleStore(s => s.loadMore)
  const invalidate = useTitleStore(s => s.invalidate)

  // Strips state
  const [continueWatching, setContinueWatching] = useState<ContinueWatchingTitle[] | null>(null)
  const [upcoming, setUpcoming] = useState<UpcomingTitle[] | null>(null)
  const cwAbortRef = useRef<AbortController | null>(null)
  const upAbortRef = useRef<AbortController | null>(null)

  useEffect(() => () => {
    cwAbortRef.current?.abort()
    upAbortRef.current?.abort()
  }, [])

  const loadContinueWatching = useCallback(async () => {
    if (continueWatching !== null) return
    cwAbortRef.current?.abort()
    const ctrl = new AbortController()
    cwAbortRef.current = ctrl
    try {
      const data = await apiFetch<ContinueWatchingTitle[]>('/titles/continue-watching', { signal: ctrl.signal })
      if (!ctrl.signal.aborted) setContinueWatching(data)
    } catch (err) {
      if (ctrl.signal.aborted) return
      throw err
    }
  }, [continueWatching])

  const loadUpcoming = useCallback(async () => {
    if (upcoming !== null) return
    upAbortRef.current?.abort()
    const ctrl = new AbortController()
    upAbortRef.current = ctrl
    try {
      const data = await apiFetch<UpcomingTitle[]>('/titles/upcoming', { signal: ctrl.signal })
      if (!ctrl.signal.aborted) setUpcoming(data)
    } catch (err) {
      if (ctrl.signal.aborted) return
      throw err
    }
  }, [upcoming])

  // Bulk selection state
  const [selecting, setSelecting] = useState(false)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [statusSheetOpen, setStatusSheetOpen] = useState(false)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)
  const [bulkPending, setBulkPending] = useState(false)
  const [bulkError, setBulkError] = useState<string | null>(null)

  function toggleSelect(id: number) {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function selectAll() {
    setSelected(new Set(titles.map(t => t.id)))
  }

  function exitSelect() {
    setSelecting(false)
    setSelected(new Set())
  }

  async function applyBulkStatus(status: string) {
    if (bulkPending) return
    setBulkPending(true)
    setBulkError(null)
    try {
      await apiFetch('/titles/batch-status', {
        method: 'POST',
        body: JSON.stringify({ ids: [...selected], status }),
      })
      setStatusSheetOpen(false)
      exitSelect()
      invalidate()
    } catch (err) {
      setBulkError(err instanceof Error ? err.message : 'Bulk status update failed')
    } finally {
      setBulkPending(false)
    }
  }

  async function confirmBulkDelete() {
    if (bulkPending) return
    setBulkPending(true)
    setBulkError(null)
    try {
      await apiFetch('/titles/batch-delete', {
        method: 'POST',
        body: JSON.stringify({ ids: [...selected] }),
      })
      exitSelect()
      invalidate()
    } catch (err) {
      setBulkError(err instanceof Error ? err.message : 'Bulk delete failed')
      throw err
    } finally {
      setBulkPending(false)
    }
  }

  // Initial fetch on mount
  useEffect(() => { fetchTitles() }, [fetchTitles])
  useEffect(() => { loadContinueWatching() }, [loadContinueWatching])
  useEffect(() => { loadUpcoming() }, [loadUpcoming])

  // Stats strip: at-a-glance figures pulled from /api/stats.
  const { data: stats } = useApi<StatsResponse>('/stats')

  // Atmospheric backdrop: prefer first continue-watching cover, else first list cover
  const backdropCover =
    continueWatching?.find(t => t.cover_url)?.cover_url
    ?? titles.find(t => t.cover_url)?.cover_url
    ?? null

  const pendingCount = counts?.pending_review ?? 0
  const unconfirmedCount = counts?.unconfirmed ?? 0
  const reviewCount = pendingCount + unconfirmedCount

  const useListView = filter.status === 'watching_behind' || filter.status === 'up_to_date'

  const statusOptions: { value: string; label: string }[] = [
    { value: 'watching', label: 'Watching' },
    { value: 'completed', label: 'Completed' },
    { value: 'dropped', label: 'Dropped' },
    { value: 'plan_to_watch', label: 'Plan to Watch' },
  ]

  return (
    <PullToRefresh onRefresh={invalidate}>
    <div className={s.page}>
      {backdropCover && (
        <div
          className={s.backdrop}
          style={{ backgroundImage: `url(${getCoverUrl(backdropCover)})` }}
          aria-hidden="true"
        />
      )}
      {/* Header */}
      <div className={s.header}>
        <div className={s.headerTitle}>Library</div>
        <button
          onClick={async () => { await apiFetch('/auth/logout', { method: 'POST' }); route(routeTo.login()) }}
          className={s.logoutBtn}
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={colors.inkDim} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
            <polyline points="16 17 21 12 16 7" />
            <line x1="21" y1="12" x2="9" y2="12" />
          </svg>
        </button>
      </div>

      {error && <ErrorBanner message={error} onRetry={invalidate} />}

      <div className={s.statsStrip}>
        <span className={s.statsStripYear}>{new Date().getFullYear()}</span>
        <span className={s.statsStripDot}>·</span>
        <span>{stats?.watched_this_year ?? 0} watched</span>
        <span className={s.statsStripDot}>·</span>
        <span>★ {stats?.avg_rating_this_year ? stats.avg_rating_this_year.toFixed(1) : '—'} avg</span>
        <span className={s.statsStripDot}>·</span>
        <span>{formatWatchtimeShort(stats?.minutes_this_week ?? 0)} this week</span>
      </div>

      {/* Section hub cards: Coming Up / In Progress / Releases */}
      <SectionCards
        cards={[
          {
            label: '// COMING UP',
            subText: upcoming === null ? undefined : `${upcoming.length} airing soon`,
            posters: upcoming ?? undefined,
            loading: upcoming === null,
            onClick: () => route(routeTo.comingUp()),
          },
          {
            label: '// IN PROGRESS',
            subText: continueWatching === null ? undefined : `${continueWatching.length} in progress`,
            posters: continueWatching ?? undefined,
            loading: continueWatching === null,
            onClick: () => route(routeTo.continueWatching()),
          },
          {
            label: '// RELEASES',
            subText: 'Explore C411',
            variant: 'accent',
            loading: false,
            onClick: () => route(routeTo.releases()),
          },
        ]}
      />

      {selecting && (
        <div className={s.selectAllRow}>
          <button className={s.selectAllBtn} onClick={selectAll}>Select all</button>
          <span className={s.selectCount}>{selected.size} of {titles.length}</span>
          <button className={s.cancelBtn} onClick={exitSelect}>Cancel</button>
        </div>
      )}

      {loading && titles.length === 0 && (
        <div className={s.posterGrid} aria-busy="true" aria-label="Loading library">
          {Array.from({ length: 9 }).map((_, i) => (
            <div key={i} className={s.skeletonTile} aria-hidden="true" />
          ))}
        </div>
      )}

      {!loading && titles.length === 0 && (
        <div className={s.centered}>
          {!filter.status ? "No titles yet. Add one with the + tab!" : "No titles in this category."}
        </div>
      )}

      {titles.length > 0 && (
        <>
          <MatchReviewBanner count={reviewCount} pendingCount={pendingCount} unconfirmedCount={unconfirmedCount} />

          {total > 0 && (
            <div className={s.counter}>
              [ {String(titles.length).padStart(3, '0')} / {String(total).padStart(3, '0')} ]
            </div>
          )}

          {useListView ? (
            <TitleList titles={titles} onUpdate={invalidate} />
          ) : (
            <div className={s.posterGrid}>
              {titles.map(t => (
                <PosterCard
                  key={t.id}
                  title={t}
                  onClick={selecting ? () => toggleSelect(t.id) : undefined}
                  onLongPress={selecting ? undefined : () => {
                    haptic(HAPTIC_SHORT)
                    setSelecting(true)
                    toggleSelect(t.id)
                  }}
                  selecting={selecting}
                  overlay={selecting && (
                    <div className={`${s.checkbox} ${selected.has(t.id) ? s.checked : ''}`}>
                      {selected.has(t.id) && '✓'}
                    </div>
                  )}
                />
              ))}
            </div>
          )}

          {hasMore && (
            <LoadMoreButton onClick={loadMore} loading={loadingMore} />
          )}
        </>
      )}

      {/* Bulk action bar */}
      {selecting && selected.size > 0 && (
        <div className={s.actionBar}>
          <span className={s.actionBarLabel}>{selected.size} selected</span>
          <button
            className={s.actionBtnStatus}
            onClick={() => setStatusSheetOpen(true)}
            disabled={bulkPending}
          >
            Status
          </button>
          <button
            className={s.actionBtnDelete}
            onClick={() => setDeleteConfirmOpen(true)}
            disabled={bulkPending}
          >
            Delete
          </button>
        </div>
      )}

      {bulkError && <ErrorBanner message={bulkError} onDismiss={() => setBulkError(null)} />}

      {/* Status picker sheet */}
      <BottomSheet open={statusSheetOpen} onClose={() => { if (!bulkPending) setStatusSheetOpen(false) }} ariaLabel="Set status for selected titles">
        <div className={s.statusSheet}>
          {statusOptions.map(opt => (
            <button
              key={opt.value}
              className={s.statusOption}
              onClick={() => applyBulkStatus(opt.value)}
              disabled={bulkPending}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </BottomSheet>

      {/* Delete confirmation */}
      <ConfirmationDrawer
        open={deleteConfirmOpen}
        onClose={() => setDeleteConfirmOpen(false)}
        onConfirm={confirmBulkDelete}
        title={`Delete ${selected.size} title${selected.size > 1 ? 's' : ''}?`}
        description="This cannot be undone."
        confirmText="Delete"
        isDangerous
      />
    </div>
    </PullToRefresh>
  )
}
