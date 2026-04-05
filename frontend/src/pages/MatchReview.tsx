import type { PaginatedResponse } from '../types'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { MatchReviewCard } from '../components/MatchReviewCard'
import { ErrorBanner } from '../components/ErrorBanner'
import clsx from 'clsx'
import s from './MatchReview.module.css'

export function MatchReview({ path }: { path?: string }) {
  const { data: pendingData, loading: l1, error: e1, mutate: m1 } = useApi<PaginatedResponse>('/titles?match_status=pending_review&limit=500')
  const { data: unconfirmedData, loading: l2, error: e2, mutate: m2 } = useApi<PaginatedResponse>('/titles?match_status=unconfirmed&limit=500')

  const loading = l1 || l2
  const error = e1 || e2
  const mutate = () => { m1(); m2() }

  const pending = pendingData?.titles ?? []
  const unconfirmed = unconfirmedData?.titles ?? []
  const titles = [...unconfirmed, ...pending]

  const handleBatchConfirm = async () => {
    for (const t of titles) {
      await apiFetch(`/titles/${t.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ match_status: 'confirmed' }),
      })
    }
    mutate()
  }

  return (
    <div className={s.page}>
      {/* Header */}
      <div className={s.header}>
        <div className={s.headerLeft}>
          <button
            onClick={() => history.back()}
            aria-label="Retour"
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
          Aucun titre à vérifier
        </div>
      )}

      {/* Unconfirmed section */}
      {unconfirmed.length > 0 && (
        <>
          <div className={clsx(s.sectionLabel, s.sectionLabelUnconfirmed)}>
            Unconfirmed ({unconfirmed.length})
          </div>
          <div className={clsx(s.cardList, s.cardListSpaced)}>
            {unconfirmed.map((t) => <MatchReviewCard key={t.id} title={t} onUpdate={mutate} />)}
          </div>
        </>
      )}

      {/* Pending section */}
      {pending.length > 0 && (
        <>
          <div className={clsx(s.sectionLabel, s.sectionLabelPending)}>
            Pending review ({pending.length})
          </div>
          <div className={s.cardList}>
            {pending.map((t) => <MatchReviewCard key={t.id} title={t} onUpdate={mutate} />)}
          </div>
        </>
      )}
    </div>
  )
}
