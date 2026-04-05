import { useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import type { Title } from '../types'
import { colors } from '../theme'
import { useTitleStore } from '../store'
import type { FilterTab } from '../components/FilterBar'
import { TitleCard } from '../components/TitleCard'
import { PosterCard } from '../components/PosterCard'
import { ErrorBanner } from '../components/ErrorBanner'
import s from './Library.module.css'

const tabToStatus: Record<FilterTab, string | undefined> = {
  all: undefined,
  watching: 'watching_behind',
  up_to_date: 'up_to_date',
  completed: 'completed',
  dropped: 'dropped',
  plan: 'plan_to_watch',
}

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
    <div onClick={() => route('/match-review')} className={s.bannerWrapper}>
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
    </div>
  )
}

function LoadMoreButton({ onClick, loading }: { onClick: () => void; loading: boolean }) {
  return (
    <div className={s.loadMoreWrapper}>
      <button onClick={onClick} disabled={loading} className={s.loadMoreBtn}>
        {loading ? 'Chargement...' : 'Charger plus'}
      </button>
    </div>
  )
}

export function Library({ filterTab: tab = 'all' }: { path?: string; filterTab?: FilterTab }) {
  const {
    titles, total, hasMore, counts,
    loading, loadingMore, error,
    setFilter, loadMore, invalidate,
  } = useTitleStore()

  // Sync tab with store filter
  useEffect(() => {
    const status = tabToStatus[tab]
    setFilter({ status, search: undefined })
  }, [tab])

  const pendingCount = counts?.pending_review ?? 0
  const unconfirmedCount = counts?.unconfirmed ?? 0
  const reviewCount = pendingCount + unconfirmedCount

  const useListView = tab === 'watching' || tab === 'up_to_date'

  return (
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

      {loading && (
        <div className={s.centered}>Loading...</div>
      )}

      {!loading && titles.length === 0 && (
        <div className={s.centered}>
          {tab === 'all' ? "No titles yet. Add one with the + tab!" : "No titles in this category."}
        </div>
      )}

      {!loading && titles.length > 0 && (
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
            <PosterGrid titles={titles} />
          )}

          {hasMore && (
            <LoadMoreButton onClick={loadMore} loading={loadingMore} />
          )}
        </>
      )}
    </div>
  )
}
