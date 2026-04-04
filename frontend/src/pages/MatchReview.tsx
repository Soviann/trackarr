import type { Title, PaginatedResponse } from '../types'
import { colors } from '../theme'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { MatchReviewCard } from '../components/MatchReviewCard'
import { ErrorBanner } from '../components/ErrorBanner'

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
    <div style={{ padding: '16px', paddingBottom: '36px' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
          <button
            onClick={() => history.back()}
            aria-label="Retour"
            style={{
              width: '32px', height: '32px', borderRadius: '50%',
              background: colors.bgCard,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              cursor: 'pointer', border: 'none', padding: 0,
            }}
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
            </svg>
          </button>
          <div style={{ fontSize: '17px', fontWeight: 600, color: colors.textPrimary }}>Match Review</div>
          {titles.length > 0 && (
            <div style={{
              minWidth: '20px', height: '20px', borderRadius: '10px',
              background: colors.accentCoral,
              display: 'flex', alignItems: 'center', justifyContent: 'center',
              padding: '0 6px',
            }}>
              <span style={{ fontSize: '10px', fontWeight: 700, color: '#fff' }}>{titles.length}</span>
            </div>
          )}
        </div>
        {titles.length > 1 && (
          <button
            onClick={handleBatchConfirm}
            style={{
              padding: '6px 12px',
              borderRadius: '8px',
              background: `${colors.accentGreen}1F`,
              border: `1px solid ${colors.accentGreen}33`,
              color: colors.accentGreen,
              fontSize: '11px',
              fontWeight: 600,
              cursor: 'pointer',
            }}
          >
            Confirm all
          </button>
        )}
      </div>

      {error && <ErrorBanner message={error} onRetry={mutate} />}

      {loading && (
        <div style={{ textAlign: 'center', padding: '40px 0', color: colors.textSecondary }}>Loading...</div>
      )}

      {!loading && titles.length === 0 && (
        <div style={{ textAlign: 'center', padding: '40px 0', color: colors.textSecondary }}>
          Aucun titre à vérifier
        </div>
      )}

      {/* Unconfirmed section */}
      {unconfirmed.length > 0 && (
        <>
          <div style={{
            fontSize: '10px', color: colors.accentCoral, fontWeight: 600,
            textTransform: 'uppercase', letterSpacing: '0.5px', marginBottom: '8px',
          }}>
            Unconfirmed ({unconfirmed.length})
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginBottom: '16px' }}>
            {unconfirmed.map((t) => <MatchReviewCard key={t.id} title={t} onUpdate={mutate} />)}
          </div>
        </>
      )}

      {/* Pending section */}
      {pending.length > 0 && (
        <>
          <div style={{
            fontSize: '10px', color: colors.accentAmber, fontWeight: 600,
            textTransform: 'uppercase', letterSpacing: '0.5px', marginBottom: '8px',
          }}>
            Pending review ({pending.length})
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {pending.map((t) => <MatchReviewCard key={t.id} title={t} onUpdate={mutate} />)}
          </div>
        </>
      )}
    </div>
  )
}
