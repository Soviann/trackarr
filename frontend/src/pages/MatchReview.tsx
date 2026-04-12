import type { PaginatedResponse } from '../types'
import { useState, useCallback } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { getName } from '../utils'
import { MatchReviewCard } from '../components/MatchReviewCard'
import { SwipeActions } from '../components/SwipeActions'
import type { SwipeAction } from '../components/SwipeActions'
import { ErrorBanner } from '../components/ErrorBanner'
import clsx from 'clsx'
import { PullToRefresh } from '../components/PullToRefresh'
import { route } from 'preact-router'
import s from './MatchReview.module.css'

export function MatchReview({ path }: { path?: string }) {
  const [pendingLimit, setPendingLimit] = useState(50)
  const [unconfirmedLimit, setUnconfirmedLimit] = useState(50)

  const { data: pendingData, loading: l1, error: e1, mutate: m1 } = useApi<PaginatedResponse>(`/titles?match_status=pending_review&limit=${pendingLimit}`)
  const { data: unconfirmedData, loading: l2, error: e2, mutate: m2 } = useApi<PaginatedResponse>(`/titles?match_status=unconfirmed&limit=${unconfirmedLimit}`)

  const loading = l1 || l2
  const error = e1 || e2
  const mutate = useCallback(() => { m1(); m2() }, [m1, m2])

  const pending = pendingData?.titles ?? []
  const unconfirmed = unconfirmedData?.titles ?? []
  const titles = [...unconfirmed, ...pending]

  const buildSwipeActions = useCallback((title: import('../types').Title): SwipeAction[] => {
    const name = getName(title)
    const hasAnyID = !!(title.imdb_id || title.tmdb_id || title.tvdb_id || title.anilist_id)
    return [
      {
        icon: (
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="20 6 9 17 4 12" />
          </svg>
        ),
        color: '#2ECC71',
        label: 'Confirm',
        disabled: !hasAnyID,
        onAction: async () => {
          await apiFetch(`/titles/${title.id}`, {
            method: 'PATCH',
            body: JSON.stringify({ match_status: 'confirmed' }),
          })
          mutate()
        },
      },
      {
        icon: (
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
          </svg>
        ),
        color: '#E67E22',
        label: 'Fix match',
        onAction: () => {
          route(`/admin/validate?q=${encodeURIComponent(title.original_title ?? name)}&id=${title.id}`)
        },
      },
    ]
  }, [mutate])

  const handleBatchConfirm = async () => {
    await Promise.all(titles.map(t =>
      apiFetch(`/titles/${t.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ match_status: 'confirmed' }),
      })
    ))
    mutate()
  }

  return (
    <PullToRefresh onRefresh={mutate}>
    <div className={s.page}>
      {/* Header */}
      <div className={s.header}>
        <div className={s.headerLeft}>
          <button
            onClick={() => history.back()}
            aria-label="Back"
            className={s.backBtn}
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
            </svg>
          </button>
          <div className={s.title}>Match Review</div>
          {titles.length > 0 && (
            <div className={s.badge}>
              <span className={s.badgeText}>{titles.length}</span>
            </div>
          )}
        </div>
        {titles.length > 1 && (
          <button onClick={handleBatchConfirm} className={s.confirmAllBtn}>
            Confirm all
          </button>
        )}
      </div>

      {error && <ErrorBanner message={error} onRetry={mutate} />}

      {loading && (
        <div className={s.statusMsg}>Loading...</div>
      )}

      {!loading && titles.length === 0 && (
        <div className={s.statusMsg}>
          No titles to review
        </div>
      )}

      {/* Unconfirmed section */}
      {unconfirmed.length > 0 && (
        <>
          <div className={clsx(s.sectionLabel, s.sectionLabelUnconfirmed)}>
            Unconfirmed ({unconfirmedData?.total ?? unconfirmed.length})
          </div>
          <div className={clsx(s.cardList, s.cardListSpaced)}>
            {unconfirmed.map((t) => (
              <SwipeActions key={t.id} actions={buildSwipeActions(t)}>
                <MatchReviewCard title={t} onUpdate={mutate} />
              </SwipeActions>
            ))}
          </div>
          {unconfirmedData?.has_more && (
            <div className={s.loadMoreRow}>
              <button className={s.loadMoreBtn} disabled={l2} onClick={() => setUnconfirmedLimit(l => l + 50)}>
                {l2 ? 'Loading...' : 'Load more'}
              </button>
            </div>
          )}
        </>
      )}

      {/* Pending section */}
      {pending.length > 0 && (
        <>
          <div className={clsx(s.sectionLabel, s.sectionLabelPending)}>
            Pending review ({pendingData?.total ?? pending.length})
          </div>
          <div className={s.cardList}>
            {pending.map((t) => (
              <SwipeActions key={t.id} actions={buildSwipeActions(t)}>
                <MatchReviewCard title={t} onUpdate={mutate} />
              </SwipeActions>
            ))}
          </div>
          {pendingData?.has_more && (
            <div className={s.loadMoreRow}>
              <button className={s.loadMoreBtn} disabled={l1} onClick={() => setPendingLimit(l => l + 50)}>
                {l1 ? 'Loading...' : 'Load more'}
              </button>
            </div>
          )}
        </>
      )}
    </div>
    </PullToRefresh>
  )
}
