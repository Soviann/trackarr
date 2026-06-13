import { useState } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { colors } from '../theme'
import { ErrorBanner } from '../components/ErrorBanner'
import type { SeasonAuditProposal } from '../types'
import s from './AdminSeasonAudit.module.css'

interface SeasonAuditResponse {
  proposals: SeasonAuditProposal[]
}

export function AdminSeasonAudit({ path }: { path?: string }) {
  const { data, loading, error, mutate } = useApi<SeasonAuditResponse>('/admin/season-audit')
  const [busyId, setBusyId] = useState<number | null>(null)
  const [busyAll, setBusyAll] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const proposals = data?.proposals ?? []

  // Low-level post — no mutate, so callers control when to refresh.
  const postAccept = async (p: SeasonAuditProposal) => {
    await apiFetch('/admin/season-audit/accept', {
      method: 'POST',
      body: JSON.stringify({ source_title_id: p.source_title_id, target_title_id: p.target_title_id, season_number: p.season_number }),
    })
  }

  const accept = async (p: SeasonAuditProposal) => {
    setBusyId(p.source_title_id)
    setActionError(null)
    try {
      await postAccept(p)
      mutate()
    } catch (e) {
      setActionError(String(e))
    } finally {
      setBusyId(null)
    }
  }

  const dismiss = async (p: SeasonAuditProposal) => {
    setBusyId(p.source_title_id)
    setActionError(null)
    try {
      await apiFetch('/admin/season-audit/dismiss', {
        method: 'POST',
        body: JSON.stringify({ source_title_id: p.source_title_id, target_title_id: p.target_title_id }),
      })
      mutate()
    } catch (e) {
      setActionError(String(e))
    } finally {
      setBusyId(null)
    }
  }

  const acceptAll = async () => {
    setBusyAll(true)
    setActionError(null)
    try {
      // Sequential awaits are intentional: each accept performs a destructive merge; serializing avoids concurrent write conflicts.
      for (const p of proposals) {
        await postAccept(p)
      }
      mutate()
    } catch (e) {
      setActionError(String(e))
    } finally {
      setBusyAll(false)
    }
  }

  return (
    <div className={s.page}>
      <div className={s.header}>
        <div className={s.headerLeft}>
          <button type="button" onClick={() => history.back()} className={s.backBtn} aria-label="Back">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={colors.ink} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
            </svg>
          </button>
          <h1 className={s.title}>Season audit</h1>
        </div>
        <button
          className={s.scanBtn}
          onClick={() => mutate()}
          disabled={loading || busyAll || busyId !== null}
        >
          Rescan
        </button>
      </div>

      {loading && <div className={s.loading}>Scanning...</div>}

      {error && <ErrorBanner message={error} onRetry={mutate} />}

      {actionError && <ErrorBanner message={actionError} onDismiss={() => setActionError(null)} />}

      {!loading && !error && proposals.length === 0 && (
        <div className={s.empty}>No season conflicts found 🎉</div>
      )}

      {!loading && !error && proposals.length > 0 && (
        <>
          {proposals.length > 1 && (
            <div className={s.topActions}>
              <button
                className={s.acceptAllBtn}
                onClick={acceptAll}
                disabled={busyAll || busyId !== null}
              >
                Accept all ({proposals.length})
              </button>
            </div>
          )}

          <section className={s.section}>
            {proposals.map((p) => {
              const seasonLabel = p.season_number === 0 ? 'Season ?' : `Season ${p.season_number}`
              const isThisBusy = busyId === p.source_title_id
              return (
                <div key={`${p.source_title_id}-${p.target_title_id}`} className={s.card}>
                  <div className={s.cardBody}>
                    <div className={s.proposal}>
                      <span className={s.sourceName}>"{p.source_name}"</span>
                      <span className={s.arrow}>→</span>
                      <span className={s.targetInfo}>{seasonLabel} of "{p.target_name}"</span>
                    </div>
                    <div className={s.sharedIdChip}>{p.shared_id}</div>
                  </div>
                  <div className={s.cardActions}>
                    <button
                      className={s.acceptBtn}
                      onClick={() => accept(p)}
                      disabled={isThisBusy || busyAll}
                    >
                      Accept
                    </button>
                    <button
                      className={s.dismissBtn}
                      onClick={() => dismiss(p)}
                      disabled={isThisBusy || busyAll}
                    >
                      Dismiss
                    </button>
                  </div>
                </div>
              )
            })}
          </section>
        </>
      )}
    </div>
  )
}
