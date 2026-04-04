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
    <div style={{ padding: '0 16px 6px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
      {titles.map((t) => <TitleCard key={t.id} title={t} onUpdate={onUpdate} />)}
    </div>
  )
}

function PosterGrid({ titles }: { titles: Title[] }) {
  if (titles.length === 0) return null
  return (
    <div style={{
      padding: '0 16px 8px',
      display: 'grid',
      gridTemplateColumns: '1fr 1fr 1fr',
      gap: '8px',
    }}>
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
    <div
      onClick={() => route('/match-review')}
      style={{
        padding: '0 16px 12px',
        cursor: 'pointer',
      }}
    >
      <div style={{
        background: 'rgba(235,87,87,0.08)',
        border: '1px solid rgba(235,87,87,0.2)',
        borderRadius: '10px',
        padding: '10px 12px',
        display: 'flex',
        alignItems: 'center',
        gap: '10px',
      }}>
        <div style={{
          width: '24px',
          height: '24px',
          borderRadius: '50%',
          background: colors.accentCoral,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
        }}>
          <span style={{ fontSize: '11px', fontWeight: 700, color: '#fff' }}>{count}</span>
        </div>
        <div style={{ flex: 1 }}>
          <span style={{ fontSize: '12px', color: colors.textPrimary, fontWeight: 500 }}>
            {count} title{count > 1 ? 's' : ''} need{count === 1 ? 's' : ''} review
          </span>
          <span style={{ fontSize: '10px', color: colors.textSecondary, marginLeft: '6px' }}>
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
    <div style={{ padding: '12px 16px 24px', textAlign: 'center' }}>
      <button
        onClick={onClick}
        disabled={loading}
        style={{
          background: colors.bgCard,
          border: `1px solid ${colors.borderCard}`,
          borderRadius: '10px',
          padding: '10px 24px',
          color: colors.accentTeal,
          fontSize: '12px',
          fontWeight: 600,
          cursor: loading ? 'default' : 'pointer',
          opacity: loading ? 0.6 : 1,
          fontFamily: 'inherit',
        }}
      >
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
    <div style={{ paddingBottom: '36px' }}>
      {/* Header */}
      <div style={{ padding: '16px 16px 10px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <div style={{ fontSize: '20px', fontWeight: 700, color: '#fff' }}>Library</div>
        <button
          onClick={async () => { await apiFetch('/auth/logout', { method: 'POST' }); route('/login') }}
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            padding: '4px',
          }}
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
        <div style={{ padding: '40px 16px', textAlign: 'center', color: colors.textSecondary }}>
          Loading...
        </div>
      )}

      {!loading && titles.length === 0 && (
        <div style={{ padding: '40px 16px', textAlign: 'center', color: colors.textSecondary }}>
          {tab === 'all' ? "No titles yet. Add one with the + tab!" : "No titles in this category."}
        </div>
      )}

      {!loading && titles.length > 0 && (
        <>
          <MatchReviewBanner count={reviewCount} pendingCount={pendingCount} unconfirmedCount={unconfirmedCount} />

          {total > 0 && (
            <div style={{ padding: '0 16px 8px' }}>
              <span style={{ fontSize: '10px', color: colors.textMuted }}>
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
