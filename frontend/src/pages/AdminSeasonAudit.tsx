import { useState } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { useTranslation } from '../i18n'
import { AdminHeader } from '../components/AdminHeader'
import { ErrorBanner } from '../components/ErrorBanner'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import { CoverImage } from '../components/CoverImage'
import { BottomSheet } from '../components/BottomSheet'
import type { SeasonAuditProposal } from '../types'
import s from './AdminSeasonAudit.module.css'

interface SeasonAuditResponse {
  proposals: SeasonAuditProposal[]
}

export function AdminSeasonAudit({ path }: { path?: string }) {
  const { t } = useTranslation()
  const { data, loading, error, mutate } = useApi<SeasonAuditResponse>('/admin/season-audit')
  const [selectedProposal, setSelectedProposal] = useState<SeasonAuditProposal | null>(null)
  const [seasonNumberInput, setSeasonNumberInput] = useState<number>(1)
  const [isMergingSingle, setIsMergingSingle] = useState(false)
  const [singleMergeError, setSingleMergeError] = useState<string | null>(null)

  const [busyDismissId, setBusyDismissId] = useState<number | null>(null)
  const [busyAll, setBusyAll] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)
  const [confirmMergeAllOpen, setConfirmMergeAllOpen] = useState(false)

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
      <AdminHeader title={t('seasonAudit.title')}>
        <button
          type="button"
          className={s.scanBtn}
          onClick={() => mutate()}
          disabled={loading || busyAll || busyDismissId !== null}
        >
          {t('seasonAudit.rescan')}
        </button>
      </AdminHeader>

      {loading && <div className={s.loading}>{t('seasonAudit.scanning')}</div>}

      {error && <ErrorBanner message={error} onRetry={mutate} />}

      {actionError && <ErrorBanner message={actionError} onDismiss={() => setActionError(null)} />}

      {!loading && !error && proposals.length === 0 && (
        <div className={s.empty}>{t('seasonAudit.empty')}</div>
      )}

      {!loading && !error && proposals.length > 0 && (
        <>
          {proposals.length > 1 && (
            <div className={s.topActions}>
              <div className={s.mergeAllContainer}>
                <button
                  type="button"
                  className={s.acceptAllBtn}
                  onClick={() => setConfirmMergeAllOpen(true)}
                  disabled={busyAll || busyDismissId !== null || hasAmbiguous}
                  title={hasAmbiguous ? t('seasonAudit.ambiguousNotice') : undefined}
                >
                  {t('seasonAudit.mergeAll', { count: String(proposals.length) })}
                </button>
                {hasAmbiguous && (
                  <span className={s.ambiguousNotice}>
                    {t('seasonAudit.ambiguousNotice')}
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
                    {p.shared_id && <div className={s.sharedIdChip}>{p.shared_id}</div>}
                    {isSuggested ? (
                      <span className={s.seasonBadgeSuggested}>
                        {t('seasonAudit.badgeSuggested', { num: String(p.season_number) })}
                      </span>
                    ) : (
                      <span className={s.seasonBadgeManual}>
                        {t('seasonAudit.badgeManual')}
                      </span>
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
                        <span className={s.entityLabel}>{t('seasonAudit.sourceLabel')}</span>
                        <strong className={s.entityTitle}>{p.source_name}</strong>
                        <span className={s.entityMeta}>
                          {p.source_year ? `${p.source_year} • ` : ''}
                          {p.source_seasons_count === 1
                            ? t('seasonAudit.seasonCount', { count: '1' })
                            : t('seasonAudit.seasonCountPlural', { count: String(p.source_seasons_count) })}
                        </span>
                      </div>
                    </div>

                    <div className={s.diffConnector}>
                      <div className={s.connectorLine} />
                      <div className={s.connectorBadge}>
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                          <polyline points="9 18 15 12 9 6" />
                        </svg>
                        <span>
                          {isSuggested
                            ? t('seasonAudit.diffConnectorSeason', { num: String(p.season_number) })
                            : t('seasonAudit.diffConnectorMerge')}
                        </span>
                      </div>
                      <div className={s.connectorLine} />
                    </div>

                    <div className={s.entitySide}>
                      <CoverImage
                        coverUrl={p.target_cover_url}
                        type="series"
                        className={s.poster}
                        alt={p.target_name}
                      />
                      <div className={s.entityInfo}>
                        <span className={s.entityLabel}>{t('seasonAudit.targetLabel')}</span>
                        <strong className={s.entityTitle}>{p.target_name}</strong>
                        <span className={s.entityMeta}>
                          {p.target_year ? `${p.target_year} • ` : ''}
                          {p.target_seasons_count === 1
                            ? t('seasonAudit.seasonCount', { count: '1' })
                            : t('seasonAudit.seasonCountPlural', { count: String(p.target_seasons_count) })}
                        </span>
                      </div>
                    </div>
                  </div>

                  <div className={s.cardActions}>
                    <button
                      type="button"
                      className={s.acceptBtn}
                      onClick={() => openMergeDrawer(p)}
                      disabled={isThisBusy || busyAll}
                    >
                      {t('seasonAudit.btnMerge')}
                    </button>
                    <button
                      type="button"
                      className={s.dismissBtn}
                      onClick={() => dismiss(p)}
                      disabled={isThisBusy || busyAll}
                    >
                      {isThisBusy ? t('common.loading') : t('seasonAudit.btnDismiss')}
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
        ariaLabel={t('seasonAudit.drawerTitle')}
      >
        {selectedProposal && (
          <div className={s.mergeDrawer}>
            <div className={s.mergeTitle}>{t('seasonAudit.drawerTitle')}</div>
            <div className={s.mergeDesc}>
              {t('seasonAudit.drawerDesc', {
                source: selectedProposal.source_name,
                target: selectedProposal.target_name,
              })}
            </div>

            <div className={s.seasonInputGroup}>
              <label htmlFor="audit-target-season" className={s.seasonLabel}>
                {t('seasonAudit.seasonLabel')}
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
                type="button"
                className={s.cancelBtn}
                onClick={closeMergeDrawer}
                disabled={isMergingSingle}
              >
                {t('common.cancel')}
              </button>
              <button
                type="button"
                className={s.confirmBtn}
                onClick={handleSingleMerge}
                disabled={isMergingSingle || seasonNumberInput < 1}
              >
                {isMergingSingle ? t('common.loading') : t('seasonAudit.confirmBtn')}
              </button>
            </div>
          </div>
        )}
      </BottomSheet>

      <ConfirmationDrawer
        open={confirmMergeAllOpen}
        onClose={() => setConfirmMergeAllOpen(false)}
        onConfirm={acceptAll}
        title={t('seasonAudit.confirmMergeAllTitle')}
        description={t('seasonAudit.confirmMergeAllDesc', { count: String(proposals.length) })}
        confirmText={t('seasonAudit.mergeAll', { count: String(proposals.length) })}
        cancelText={t('common.cancel')}
        isDangerous
      />
    </div>
  )
}
