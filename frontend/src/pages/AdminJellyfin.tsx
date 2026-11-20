import type { JSX } from 'preact'
import { useState, useEffect } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { PullToRefresh } from '../components/PullToRefresh'
import { AdminHeader } from '../components/AdminHeader'
import { formatDateTime24h } from '../utils'
import { useTranslation } from '../i18n'
import type { Settings } from '../types'
import s from './AdminJellyfin.module.css'

interface MediaServerSystemSettings {
  jellyfin_webhook_secret?: string
  jellyfin_webhook_url?: string
  jellyfin_url?: string
  jellyfin_api_key?: string
  jellyfin_user_id?: string
  jellyfin_api_configured?: boolean
  plex_webhook_secret?: string
  plex_webhook_url?: string
}

interface JellyfinSyncResult {
  dry_run: boolean
  matched: number
  updated: number
  skipped: number
  missing: number
  errors: number
}

export function AdminJellyfin({ path }: { path?: string }): JSX.Element {
  const { t } = useTranslation()
  const { data: settings, mutate: refetchSettings } = useApi<Settings>('/settings')
  const { data: sysSettings, mutate: refetchSysSettings } = useApi<MediaServerSystemSettings>('/admin/system-settings')

  const [jellyfinSecret, setJellyfinSecret] = useState('')
  const [jellyfinUrl, setJellyfinUrl] = useState('')
  const [jellyfinApiKey, setJellyfinApiKey] = useState('')
  const [jellyfinUserId, setJellyfinUserId] = useState('')
  const [plexSecret, setPlexSecret] = useState('')
  const [saving, setSaving] = useState(false)
  const [testingJf, setTestingJf] = useState(false)
  const [jfTestStatus, setJfTestStatus] = useState<{ ok: boolean; text: string } | null>(null)
  const [syncingJf, setSyncingJf] = useState(false)
  const [dryRunJf, setDryRunJf] = useState(false)
  const [jfSyncResult, setJfSyncResult] = useState<JellyfinSyncResult | null>(null)
  const [copiedKey, setCopiedKey] = useState<'jellyfin' | 'plex' | null>(null)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  useEffect(() => {
    if (sysSettings) {
      setJellyfinSecret(sysSettings.jellyfin_webhook_secret || '')
      setJellyfinUrl(sysSettings.jellyfin_url || '')
      setJellyfinApiKey(sysSettings.jellyfin_api_key || '')
      setJellyfinUserId(sysSettings.jellyfin_user_id || '')
      setPlexSecret(sysSettings.plex_webhook_secret || '')
    }
  }, [sysSettings])

  const handleRefresh = async () => {
    await Promise.all([refetchSettings(), refetchSysSettings()])
  }

  const handleCopy = (server: 'jellyfin' | 'plex', url: string) => {
    if (!url || !navigator.clipboard) return
    navigator.clipboard
      .writeText(url)
      .then(() => {
        setCopiedKey(server)
        setTimeout(() => setCopiedKey(null), 3000)
      })
      .catch(() => {})
  }

  const handleSave = async (e?: Event) => {
    if (e) e.preventDefault()
    setSaving(true)
    setSuccessMsg(null)
    setErrorMsg(null)

    try {
      await apiFetch('/admin/system-settings', {
        method: 'PUT',
        body: JSON.stringify({
          jellyfin_webhook_secret: jellyfinSecret,
          jellyfin_url: jellyfinUrl,
          jellyfin_api_key: jellyfinApiKey,
          jellyfin_user_id: jellyfinUserId,
          plex_webhook_secret: plexSecret,
        }),
      })
      setSuccessMsg(t('mediaServers.savedSuccess'))
      await handleRefresh()
      setTimeout(() => setSuccessMsg(null), 4000)
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  const handleTestJellyfin = async () => {
    setTestingJf(true)
    setJfTestStatus(null)
    try {
      const res = await apiFetch<{ ok: boolean; message?: string; error?: string }>(
        '/admin/system-settings/test/jellyfin',
        {
          method: 'POST',
          body: JSON.stringify({
            jellyfin_url: jellyfinUrl,
            jellyfin_api_key: jellyfinApiKey,
            jellyfin_user_id: jellyfinUserId,
          }),
        }
      )
      setJfTestStatus({
        ok: res.ok,
        text: res.ok ? res.message || 'OK' : res.error || 'Error',
      })
    } catch (err: unknown) {
      setJfTestStatus({ ok: false, text: err instanceof Error ? err.message : String(err) })
    } finally {
      setTestingJf(false)
    }
  }

  const handleSyncJellyfin = async () => {
    setSyncingJf(true)
    setJfSyncResult(null)
    setErrorMsg(null)
    try {
      const res = await apiFetch<JellyfinSyncResult>('/admin/jellyfin/sync-ratings', {
        method: 'POST',
        body: JSON.stringify({ dry_run: dryRunJf }),
      })
      setJfSyncResult(res)
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
    } finally {
      setSyncingJf(false)
    }
  }

  const isJellyfinConfigured = Boolean(jellyfinSecret || settings?.jellyfin_configured)
  const isPlexConfigured = Boolean(plexSecret || settings?.plex_last_scrobble_at)

  const jellyfinLastScrobble = settings?.jellyfin_last_scrobble_at
    ? formatDateTime24h(settings.jellyfin_last_scrobble_at)
    : t('mediaServers.noneRecorded')

  const plexLastScrobble = settings?.plex_last_scrobble_at
    ? formatDateTime24h(settings.plex_last_scrobble_at)
    : t('mediaServers.noneRecorded')

  const isLoading = !settings && !sysSettings

  return (
    <PullToRefresh onRefresh={handleRefresh}>
      <div className={s.page}>
        <AdminHeader title={t('mediaServers.title')}>
          <button
            type="button"
            className={s.saveBtn}
            onClick={() => void handleSave()}
            disabled={saving}
          >
            {saving ? t('mediaServers.saving') : t('mediaServers.saveSettings')}
          </button>
        </AdminHeader>

        <p className={s.introText}>{t('mediaServers.desc')}</p>

        {successMsg && (
          <div className={s.alertSuccess} role="status">
            {successMsg}
          </div>
        )}
        {errorMsg && (
          <div className={s.alertError} role="alert">
            {errorMsg}
          </div>
        )}

        {isLoading ? (
          <div className={s.loading}>{t('common.loading')}</div>
        ) : (
          <>
            {/* 1. JELLYFIN */}
            <div className={s.section}>
              <div className={s.sectionHeader}>
                <div className={s.sectionTitleRow}>
                  <h2 className={s.sectionTitle}>
                    <span>🟣 {t('mediaServers.jellyfinTitle')}</span>
                  </h2>
                  <span
                    className={`${s.statusBadge} ${
                      isJellyfinConfigured ? s.statusOk : s.statusMissing
                    }`}
                  >
                    {isJellyfinConfigured
                      ? t('mediaServers.configured')
                      : t('mediaServers.missingSecret')}
                  </span>
                </div>
              </div>

              <div className={s.cardBody}>
                {/* Secret Token */}
                <div className={s.fieldGroup}>
                  <label htmlFor="jellyfin_webhook_secret" className={s.label}>
                    <span>{t('mediaServers.secretLabel', { server: 'Jellyfin' })}</span>
                  </label>
                  <input
                    id="jellyfin_webhook_secret"
                    name="jellyfin_webhook_secret"
                    type="text"
                    autoComplete="off"
                    value={jellyfinSecret}
                    onInput={(e) => setJellyfinSecret((e.target as HTMLInputElement).value)}
                    placeholder={t('mediaServers.secretPlaceholder', { server: 'Jellyfin' })}
                    className={`${s.input} ${s.inputCode}`}
                  />
                </div>

                {/* Webhook URL with Copy button */}
                <div className={s.fieldGroup}>
                  <label htmlFor="jellyfin_webhook_url" className={s.label}>
                    <span>{t('mediaServers.webhookUrlLabel')}</span>
                  </label>
                  <div className={s.fieldRow}>
                    <input
                      id="jellyfin_webhook_url"
                      name="jellyfin_webhook_url"
                      type="text"
                      readOnly
                      value={sysSettings?.jellyfin_webhook_url || ''}
                      placeholder="https://<your-domain>/api/webhook/jellyfin/<secret>"
                      className={`${s.input} ${s.inputCode}`}
                    />
                    {sysSettings?.jellyfin_webhook_url && (
                      <button
                        type="button"
                        onClick={() => handleCopy('jellyfin', sysSettings.jellyfin_webhook_url || '')}
                        className={s.actionBtn}
                        aria-label={copiedKey === 'jellyfin' ? t('mediaServers.copied') : t('mediaServers.copyUrl')}
                      >
                        {copiedKey === 'jellyfin' ? t('mediaServers.copied') : t('mediaServers.copyUrl')}
                      </button>
                    )}
                  </div>
                </div>

                {/* Last Scrobble Stat */}
                <div className={s.statRow}>
                  <div className={s.statInfo}>
                    <div className={s.statLabel}>{t('mediaServers.lastScrobbleLabel')}</div>
                    <div className={s.statDesc}>
                      {t('mediaServers.lastScrobbleDesc', { server: 'Jellyfin' })}
                    </div>
                  </div>
                  <span
                    className={`${s.statValue} ${
                      settings?.jellyfin_last_scrobble_at ? s.statValueActive : ''
                    }`}
                  >
                    {jellyfinLastScrobble}
                  </span>
                </div>

                {/* Outbound API Config */}
                <div className={s.fieldGroup}>
                  <h3 className={s.helpTitle}>{t('mediaServers.jellyfinApiSectionTitle')}</h3>
                  <label htmlFor="jellyfin_url" className={s.label}>
                    <span>{t('mediaServers.jellyfinUrlLabel')}</span>
                  </label>
                  <input
                    id="jellyfin_url"
                    name="jellyfin_url"
                    type="url"
                    value={jellyfinUrl}
                    onInput={(e) => setJellyfinUrl((e.target as HTMLInputElement).value)}
                    placeholder="http://localhost:8096"
                    className={`${s.input} ${s.inputCode}`}
                  />
                </div>
                <div className={s.fieldGroup}>
                  <label htmlFor="jellyfin_api_key" className={s.label}>
                    <span>{t('mediaServers.jellyfinApiKeyLabel')}</span>
                  </label>
                  <input
                    id="jellyfin_api_key"
                    name="jellyfin_api_key"
                    type="password"
                    autoComplete="off"
                    value={jellyfinApiKey}
                    onInput={(e) => setJellyfinApiKey((e.target as HTMLInputElement).value)}
                    className={`${s.input} ${s.inputCode}`}
                  />
                </div>
                <div className={s.fieldGroup}>
                  <label htmlFor="jellyfin_user_id" className={s.label}>
                    <span>{t('mediaServers.jellyfinUserIdLabel')}</span>
                  </label>
                  <input
                    id="jellyfin_user_id"
                    name="jellyfin_user_id"
                    type="text"
                    autoComplete="off"
                    value={jellyfinUserId}
                    onInput={(e) => setJellyfinUserId((e.target as HTMLInputElement).value)}
                    className={`${s.input} ${s.inputCode}`}
                  />
                </div>
                <div className={s.fieldGroup}>
                  <div className={s.fieldRow}>
                    <button
                      type="button"
                      className={s.actionBtn}
                      onClick={() => void handleTestJellyfin()}
                      disabled={testingJf}
                    >
                      {testingJf ? t('mediaServers.testingConnection') : t('mediaServers.testConnection')}
                    </button>
                    <button
                      type="button"
                      className={s.actionBtn}
                      onClick={() => void handleSyncJellyfin()}
                      disabled={syncingJf || !sysSettings?.jellyfin_api_configured}
                    >
                      {syncingJf ? t('mediaServers.syncingRatings') : t('mediaServers.syncAllRatings')}
                    </button>
                  </div>
                  <label className={s.label} style={{ marginTop: '0.5rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                    <input
                      type="checkbox"
                      checked={dryRunJf}
                      onChange={(e) => setDryRunJf((e.target as HTMLInputElement).checked)}
                    />
                    <span>{t('mediaServers.dryRunLabel')}</span>
                  </label>
                  {jfTestStatus && (
                    <div role="status" className={jfTestStatus.ok ? s.alertSuccess : s.alertError} style={{ marginTop: '0.5rem' }}>
                      {jfTestStatus.ok ? `✅ ${jfTestStatus.text}` : `❌ ${jfTestStatus.text}`}
                    </div>
                  )}
                  {jfSyncResult && (
                    <div role="status" className={s.alertSuccess} style={{ marginTop: '0.5rem' }}>
                      {t('mediaServers.syncSummary', {
                        matched: jfSyncResult.matched,
                        updated: jfSyncResult.updated,
                        skipped: jfSyncResult.skipped,
                        missing: jfSyncResult.missing,
                        errors: jfSyncResult.errors,
                      })}
                    </div>
                  )}
                </div>

                {/* Instructions */}
                <div className={s.helpCard}>
                  <h3 className={s.helpTitle}>{t('mediaServers.jellyfinHelpTitle')}</h3>
                  <p className={s.helpText}>{t('mediaServers.jellyfinHelpStep1')}</p>
                  <div className={s.helpCode}>
                    <strong>{t('mediaServers.webhookUrlLabel')}:</strong>{' '}
                    {sysSettings?.jellyfin_webhook_url || 'https://<your-domain>/api/webhook/jellyfin/<secret>'}
                    <br />
                    {t('mediaServers.jellyfinNotificationType')}
                    <br />
                    {t('mediaServers.jellyfinItemType')}
                  </div>
                </div>
              </div>
            </div>

            {/* 2. PLEX */}
            <div className={s.section}>
              <div className={s.sectionHeader}>
                <div className={s.sectionTitleRow}>
                  <h2 className={s.sectionTitle}>
                    <span>🟠 {t('mediaServers.plexTitle')}</span>
                  </h2>
                  <span
                    className={`${s.statusBadge} ${
                      isPlexConfigured ? s.statusOk : s.statusMissing
                    }`}
                  >
                    {isPlexConfigured
                      ? t('mediaServers.configured')
                      : t('mediaServers.missingSecret')}
                  </span>
                </div>
              </div>

              <div className={s.cardBody}>
                {/* Secret Token */}
                <div className={s.fieldGroup}>
                  <label htmlFor="plex_webhook_secret" className={s.label}>
                    <span>{t('mediaServers.secretLabel', { server: 'Plex' })}</span>
                  </label>
                  <input
                    id="plex_webhook_secret"
                    name="plex_webhook_secret"
                    type="text"
                    autoComplete="off"
                    value={plexSecret}
                    onInput={(e) => setPlexSecret((e.target as HTMLInputElement).value)}
                    placeholder={t('mediaServers.secretPlaceholder', { server: 'Plex' })}
                    className={`${s.input} ${s.inputCode}`}
                  />
                </div>

                {/* Webhook URL with Copy button */}
                <div className={s.fieldGroup}>
                  <label htmlFor="plex_webhook_url" className={s.label}>
                    <span>{t('mediaServers.webhookUrlLabel')}</span>
                  </label>
                  <div className={s.fieldRow}>
                    <input
                      id="plex_webhook_url"
                      name="plex_webhook_url"
                      type="text"
                      readOnly
                      value={sysSettings?.plex_webhook_url || ''}
                      placeholder="https://<your-domain>/api/webhook/plex/<secret>"
                      className={`${s.input} ${s.inputCode}`}
                    />
                    {sysSettings?.plex_webhook_url && (
                      <button
                        type="button"
                        onClick={() => handleCopy('plex', sysSettings.plex_webhook_url || '')}
                        className={s.actionBtn}
                        aria-label={copiedKey === 'plex' ? t('mediaServers.copied') : t('mediaServers.copyUrl')}
                      >
                        {copiedKey === 'plex' ? t('mediaServers.copied') : t('mediaServers.copyUrl')}
                      </button>
                    )}
                  </div>
                </div>

                {/* Last Scrobble Stat */}
                <div className={s.statRow}>
                  <div className={s.statInfo}>
                    <div className={s.statLabel}>{t('mediaServers.lastScrobbleLabel')}</div>
                    <div className={s.statDesc}>
                      {t('mediaServers.lastScrobbleDesc', { server: 'Plex' })}
                    </div>
                  </div>
                  <span
                    className={`${s.statValue} ${
                      settings?.plex_last_scrobble_at ? s.statValueActive : ''
                    }`}
                  >
                    {plexLastScrobble}
                  </span>
                </div>

                {/* Instructions */}
                <div className={s.helpCard}>
                  <h3 className={s.helpTitle}>{t('mediaServers.plexHelpTitle')}</h3>
                  <p className={s.helpText}>{t('mediaServers.plexHelpStep1')}</p>
                  <div className={s.helpCode}>
                    <strong>{t('mediaServers.webhookUrlLabel')}:</strong>{' '}
                    {sysSettings?.plex_webhook_url || 'https://<your-domain>/api/webhook/plex/<secret>'}
                    <br />
                    {t('mediaServers.plexPayloadDesc')}
                  </div>
                </div>
              </div>
            </div>
          </>
        )}
      </div>
    </PullToRefresh>
  )
}
