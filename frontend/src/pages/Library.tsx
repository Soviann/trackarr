import { useEffect, useState, useCallback } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import { haptic, HAPTIC_SHORT } from '../utils/haptic'
import type { Title, ContinueWatchingTitle, UpcomingTitle } from '../types'
import { colors } from '../theme'
import { useTitleStore } from '../store'
import { TitleCard } from '../components/TitleCard'
import { PosterCard } from '../components/PosterCard'
import { ErrorBanner } from '../components/ErrorBanner'
import { CollapsibleSection } from '../components/CollapsibleSection'
import { PosterStrip } from '../components/PosterStrip'
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
    <button type="button" onClick={() => route('/match-review')} className={s.bannerWrapper}>
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
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke={colors.textMuted} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
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

export function Library(_props: { path?: string }) {
  const titles = useTitleStore(s => s.titles)
  const total = useTitleStore(s => s.total)
  const hasMore = useTitleStore(s => s.hasMore)
  const counts = useTitleStore(s => s.counts)
  const filter = useTitleStore(s => s.filter)
  const loading = useTitleStore(s => s.loading)
  const loadingMore = useTitleStore(s => s.loadingMore)
  const error = useTitleStore(s => s.error)
  const fetchTitles = useTitleStore(s => s.fetchTitles)
  const loadMore = useTitleStore(s => s.loadMore)
  const invalidate = useTitleStore(s => s.invalidate)

  // Strips state
  const [continueWatching, setContinueWatching] = useState<ContinueWatchingTitle[] | null>(null)
  const [upcoming, setUpcoming] = useState<UpcomingTitle[] | null>(null)

  const loadContinueWatching = useCallback(async () => {
    if (continueWatching !== null) return
    const data = await apiFetch<ContinueWatchingTitle[]>('/titles/continue-watching')
    setContinueWatching(data)
  }, [continueWatching])

  const loadUpcoming = useCallback(async () => {
    if (upcoming !== null) return
    const data = await apiFetch<UpcomingTitle[]>('/titles/upcoming')
    setUpcoming(data)
  }, [upcoming])

  // Bulk selection state
  const [selecting, setSelecting] = useState(false)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [statusSheetOpen, setStatusSheetOpen] = useState(false)
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false)

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
    await apiFetch('/api/titles/batch-status', {
      method: 'POST',
      body: JSON.stringify({ ids: [...selected], status }),
    })
    setStatusSheetOpen(false)
    exitSelect()
    invalidate()
  }

  async function confirmBulkDelete() {
    await apiFetch('/api/titles/batch-delete', {
      method: 'POST',
      body: JSON.stringify({ ids: [...selected] }),
    })
    setDeleteConfirmOpen(false)
    exitSelect()
    invalidate()
  }

  // Initial fetch on mount
  useEffect(() => { fetchTitles() }, [fetchTitles])

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
      {/* Header */}
      <div className={s.header}>
        <div className={s.headerTitle}>Library</div>
        <button
          onClick={async () => { await apiFetch('/auth/logout', { method: 'POST' }); route('/login') }}
          className={s.logoutBtn}
        >
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke={colors.textMuted} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
            <polyline points="16 17 21 12 16 7" />
            <line x1="21" y1="12" x2="9" y2="12" />
          </svg>
        </button>
      </div>

      {error && <ErrorBanner message={error} onRetry={invalidate} />}

      {/* Collapsible strips */}
      <CollapsibleSection title="Coming up" count={upcoming?.length} onExpand={loadUpcoming}>
        {upcoming && (
          <PosterStrip items={upcoming.map(t => {
            const { label, variant } = airDateBadge(t.next_air_date)
            return { id: t.id, cover_url: t.cover_url, name: t.name, sublabel: label, sublabelVariant: variant }
          })} />
        )}
      </CollapsibleSection>

      <CollapsibleSection title="Continue Watching" count={continueWatching?.length} onExpand={loadContinueWatching}>
        {continueWatching && (
          <PosterStrip items={continueWatching.map(t => ({
            id: t.id,
            cover_url: t.cover_url,
            name: t.name,
            sublabel: t.next_air_episode ?? '',
            progressRatio: t.total_episodes > 0 ? t.watched_episodes / t.total_episodes : 0,
          }))} />
        )}
      </CollapsibleSection>

      {selecting && (
        <div className={s.selectAllRow}>
          <button className={s.selectAllBtn} onClick={selectAll}>Select all</button>
          <span className={s.selectCount}>{selected.size} of {titles.length}</span>
          <button className={s.cancelBtn} onClick={exitSelect}>Cancel</button>
        </div>
      )}

      {loading && titles.length === 0 && (
        <div className={s.centered}>Loading...</div>
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
              <span className={s.counterText}>
                {titles.length} / {total} titles
              </span>
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
          <button className={s.actionBtnStatus} onClick={() => setStatusSheetOpen(true)}>Status</button>
          <button className={s.actionBtnDelete} onClick={() => setDeleteConfirmOpen(true)}>Delete</button>
        </div>
      )}

      {/* Status picker sheet */}
      <BottomSheet open={statusSheetOpen} onClose={() => setStatusSheetOpen(false)}>
        <div className={s.statusSheet}>
          {statusOptions.map(opt => (
            <button key={opt.value} className={s.statusOption} onClick={() => applyBulkStatus(opt.value)}>
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
