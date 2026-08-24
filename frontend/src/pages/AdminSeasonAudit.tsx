import { useState } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { colors } from '../theme'
import { ErrorBanner } from '../components/ErrorBanner'
import { CoverImage } from '../components/CoverImage'
import { BottomSheet } from '../components/BottomSheet'
import type { SeasonAuditProposal } from '../types'
import s from './AdminSeasonAudit.module.css'

interface SeasonAuditResponse {
  proposals: SeasonAuditProposal[]
}

export function AdminSeasonAudit({ path }: { path?: string }) {
  const { data, loading, error, mutate } = useApi<SeasonAuditResponse>('/admin/season-audit')
  const [selectedProposal, setSelectedProposal] = useState<SeasonAuditProposal | null>(null)
  const [seasonNumberInput, setSeasonNumberInput] = useState<number>(1)
  const [isMergingSingle, setIsMergingSingle] = useState(false)
  const [singleMergeError, setSingleMergeError] = useState<string | null>(null)

  const [busyDismissId, setBusyDismissId] = useState<number | null>(null)
  const [busyAll, setBusyAll] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const proposals = data?.proposals ?? []
  const hasAmbiguous = proposals.some((p) => p.season_number <= 0)

  const openMergeDrawer = (p: SeasonAuditProposal) => {
    setSelectedProposal(p)
    setSeasonNumberInput(p.season_number > 0 ? p.season_number : (p.target_seasons_count + 1))
    setSingleMergeError(null)
  }

  const closeMergeDrawer = () => {
    if (isMergingSingle) return
    setSelectedProposal(null)
    setSingleMergeError(null)
  }

  const handleSingleMerge = async () => {
    if (!selectedProposal) return
    setIsMergingSingle(true)
    setSingleMergeError(null)
    try {
      await apiFetch('/admin/season-audit/accept', {
        method: 'POST',
        body: JSON.stringify({
          source_title_id: selectedProposal.source_title_id,
          target_title_id: selectedProposal.target_title_id,
          season_number: seasonNumberInput,
        }),
      })
      setSelectedProposal(null)
      mutate()
    } catch (e) {
      setSingleMergeError(String(e))
    } finally {
      setIsMergingSingle(false)
    }
  }

  const dismiss = async (p: SeasonAuditProposal) => {
    setBusyDismissId(p.source_title_id)
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
      setBusyDismissId(null)
    }
  }

  const acceptAll = async () => {
    if (hasAmbiguous) return
    setBusyAll(true)
    setActionError(null)
    try {
      // Sequential awaits are intentional: each accept performs a destructive merge; serializing avoids concurrent write conflicts.
      for (const p of proposals) {
        await apiFetch('/admin/season-audit/accept', {
          method: 'POST',
          body: JSON.stringify({
            source_title_id: p.source_title_id,
            target_title_id: p.target_title_id,
            season_number: p.season_number,
          }),
        })
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
          disabled={loading || busyAll || busyDismissId !== null}
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
              <div className={s.mergeAllContainer}>
                <button
                  className={s.acceptAllBtn}
                  onClick={acceptAll}
                  disabled={busyAll || busyDismissId !== null || hasAmbiguous}
                  title={hasAmbiguous ? 'Disabled: some proposals require manual season assignment' : undefined}
                >
                  Merge all ({proposals.length})
                </button>
                {hasAmbiguous && (
                  <span className={s.ambiguousNotice}>
                    Disabled: some proposals require manual season assignment
                  </span>
                )}
              </div>
            </div>
          )}

          <section className={s.section}>
            {proposals.map((p) => {
              const isSuggested = p.season_number > 0
              const isThisBusy = busyDismissId === p.source_title_id

              return (
                <div key={`${p.source_title_id}-${p.target_title_id}`} className={s.card}>
                  <div className={s.cardHeader}>
                    <div className={s.sharedIdChip}>{p.shared_id}</div>
                    {isSuggested ? (
                      <span className={s.seasonBadgeSuggested}>Season {p.season_number} suggested</span>
                    ) : (
                      <span className={s.seasonBadgeManual}>Season to define</span>
                    )}
                  </div>

                  <div className={s.cardComparison}>
                    <div className={s.entitySide}>
                      <CoverImage
                        coverUrl={p.source_cover_url}
                        type="series"
                        className={s.poster}
                        alt={p.source_name}
                      />
                      <div className={s.entityInfo}>
                        <span className={s.entityLabel}>Source (to merge)</span>
                        <strong className={s.entityTitle}>{p.source_name}</strong>
                        <span className={s.entityMeta}>
                          {p.source_year ? `${p.source_year} • ` : ''}
                          {p.source_seasons_count} season{p.source_seasons_count > 1 ? 's' : ''}
                        </span>
                      </div>
                    </div>

                    <div className={s.arrowCol}>
                      <span className={s.arrow}>➔</span>
                    </div>

                    <div className={s.entitySide}>
                      <CoverImage
                        coverUrl={p.target_cover_url}
                        type="series"
                        className={s.poster}
                        alt={p.target_name}
                      />
                      <div className={s.entityInfo}>
                        <span className={s.entityLabel}>Target (main series)</span>
                        <strong className={s.entityTitle}>{p.target_name}</strong>
                        <span className={s.entityMeta}>
                          {p.target_year ? `${p.target_year} • ` : ''}
                          {p.target_seasons_count} season{p.target_seasons_count > 1 ? 's' : ''}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div className={s.cardActions}>
                    <button
                      className={s.acceptBtn}
                      onClick={() => openMergeDrawer(p)}
                      disabled={isThisBusy || busyAll}
                    >
                      Merge...
                    </button>
                    <button
                      className={s.dismissBtn}
                      onClick={() => dismiss(p)}
                      disabled={isThisBusy || busyAll}
                    >
                      {isThisBusy ? 'Dismissing...' : 'Dismiss'}
                    </button>
                  </div>
                </div>
              )
            })}
          </section>
        </>
      )}

      <BottomSheet
        open={!!selectedProposal}
        onClose={closeMergeDrawer}
        ariaLabel="Merge season into series"
      >
        {selectedProposal && (
          <div className={s.mergeDrawer}>
            <div className={s.mergeTitle}>Merge season into series</div>
            <div className={s.mergeDesc}>
              This will merge &quot;{selectedProposal.source_name}&quot; into &quot;{selectedProposal.target_name}&quot;.
              Episodes, watched progress, and metadata will be attached under the chosen season number. The source title will be removed.
            </div>

            <div className={s.seasonInputGroup}>
              <label htmlFor="audit-target-season" className={s.seasonLabel}>
                Integrate as season number:
              </label>
              <input
                id="audit-target-season"
                type="number"
                min="1"
                value={seasonNumberInput}
                onInput={(e) => setSeasonNumberInput(Math.max(1, Number((e.target as HTMLInputElement).value)))}
                className={s.seasonInput}
              />
            </div>

            {singleMergeError && <div className={s.mergeError}>{singleMergeError}</div>}

            <div className={s.mergeActions}>
              <button
                className={s.cancelBtn}
                onClick={closeMergeDrawer}
                disabled={isMergingSingle}
              >
                Cancel
              </button>
              <button
                className={s.confirmBtn}
                onClick={handleSingleMerge}
                disabled={isMergingSingle || seasonNumberInput < 1}
              >
                {isMergingSingle ? 'Merging...' : 'Confirm merge'}
              </button>
            </div>
          </div>
        )}
      </BottomSheet>
    </div>
  )
}
