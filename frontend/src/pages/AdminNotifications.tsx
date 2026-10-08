import type { JSX } from 'preact'
import { useState, useEffect } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { PullToRefresh } from '../components/PullToRefresh'
import { AdminHeader } from '../components/AdminHeader'
import { useTranslation } from '../i18n'
import s from './AdminNotifications.module.css'

interface NotifPrefs {
  notif_rating_prompt: boolean
  notif_dead_task: boolean
  notif_series_ended: boolean
}

interface VapidSettingsResponse {
  vapid_public_key: string
  vapid_subject: string
  vapid_configured: boolean
}

export function AdminNotifications({ path }: { path?: string }): JSX.Element {
  const { t } = useTranslation()
  const { data: fetchedPrefs, mutate: refetchPrefs } = useApi<NotifPrefs>('/admin/notifications')
  const { data: fetchedSys, mutate: refetchSys } = useApi<VapidSettingsResponse>('/admin/system-settings')

  const [prefs, setPrefs] = useState<NotifPrefs | null>(null)
  const [vapidPublicKey, setVapidPublicKey] = useState('')
  const [vapidSubject, setVapidSubject] = useState('')
  const [vapidConfigured, setVapidConfigured] = useState(false)

  const [saving, setSaving] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [copiedKey, setCopiedKey] = useState(false)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  useEffect(() => {
    if (fetchedPrefs) {
      setPrefs(fetchedPrefs)
    }
  }, [fetchedPrefs])

  useEffect(() => {
    if (fetchedSys) {
      setVapidPublicKey(fetchedSys.vapid_public_key || '')
      setVapidSubject(fetchedSys.vapid_subject || '')
      setVapidConfigured(Boolean(fetchedSys.vapid_configured))
    }
  }, [fetchedSys])

  const handleRefresh = async () => {
    await Promise.all([refetchPrefs(), refetchSys()])
  }

  const toggle = async (key: keyof NotifPrefs) => {
    if (!prefs || saving) return
    const updated = { ...prefs, [key]: !prefs[key] }
    setPrefs(updated)
    setSaving(true)
    setErrorMsg(null)
    try {
      await apiFetch('/admin/notifications', {
        method: 'PUT',
        body: JSON.stringify({ [key]: updated[key] }),
      })
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
      setPrefs(prefs)
    } finally {
      setSaving(false)
    }
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
          vapid_public_key: vapidPublicKey,
          vapid_subject: vapidSubject,
        }),
      })
      if (prefs) {
        await apiFetch('/admin/notifications', {
          method: 'PUT',
          body: JSON.stringify(prefs),
        })
      }
      setSuccessMsg(t('notificationsSettings.savedSuccess'))
      await handleRefresh()
      setTimeout(() => setSuccessMsg(null), 4000)
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  const handleGenerateVAPID = async () => {
    setGenerating(true)
    setErrorMsg(null)
    try {
      const res = await apiFetch<{ ok: boolean; message?: string; vapid_public_key?: string; vapid_subject?: string }>(
        '/admin/system-settings/vapid/generate',
        {
          method: 'POST',
          body: JSON.stringify({ subject: vapidSubject }),
        }
      )
      if (res.vapid_public_key) {
        setVapidPublicKey(res.vapid_public_key)
        setVapidConfigured(true)
      }
      if (res.vapid_subject) {
        setVapidSubject(res.vapid_subject)
      }
      setSuccessMsg(res.message || t('notificationsSettings.regenerateSuccess'))
      await handleRefresh()
      setTimeout(() => setSuccessMsg(null), 4000)
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : String(err))
    } finally {
      setGenerating(false)
    }
  }

  const handleCopyKey = () => {
    if (!vapidPublicKey || !navigator.clipboard) return
    navigator.clipboard
      .writeText(vapidPublicKey)
      .then(() => {
        setCopiedKey(true)
        setTimeout(() => setCopiedKey(false), 3000)
      })
      .catch(() => {})
  }

  const notifItems = [
    {
      key: 'notif_rating_prompt' as const,
      label: t('notificationsSettings.ratingReminderLabel'),
      description: t('notificationsSettings.ratingReminderDesc'),
    },
    {
      key: 'notif_dead_task' as const,
      label: t('notificationsSettings.failedTaskLabel'),
      description: t('notificationsSettings.failedTaskDesc'),
    },
    {
      key: 'notif_series_ended' as const,
      label: t('notificationsSettings.seriesEndedLabel'),
      description: t('notificationsSettings.seriesEndedDesc'),
    },
  ]

  const isLoading = !prefs && !fetchedSys

  return (
    <PullToRefresh onRefresh={handleRefresh}>
      <div className={s.page}>
        <AdminHeader title={t('notificationsSettings.title')}>
          <button
            type="button"
            className={s.saveBtn}
            onClick={() => void handleSave()}
            disabled={saving}
          >
            {saving ? t('notificationsSettings.saving') : t('notificationsSettings.saveSettings')}
          </button>
        </AdminHeader>

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
            {/* 1. VAPID CONFIGURATION */}
            <div className={s.section}>
              <div className={s.sectionHeader}>
                <div className={s.sectionHeaderContent}>
                  <div className={s.sectionTitleRow}>
                    <h2 className={s.sectionTitle}>
                      <span>🔔 {t('notificationsSettings.vapidTitle')}</span>
                    </h2>
                    <span
                      className={`${s.statusBadge} ${
                        vapidConfigured ? s.statusOk : s.statusMissing
                      }`}
                    >
                      {vapidConfigured
                        ? t('notificationsSettings.autoConfigured')
                        : t('notificationsSettings.notConfigured')}
                    </span>
                  </div>
                  <p className={s.sectionDesc}>{t('notificationsSettings.vapidDesc')}</p>
                </div>
                <button
                  type="button"
                  disabled={generating}
                  onClick={() => void handleGenerateVAPID()}
                  className={s.actionBtn}
                >
                  {generating
                    ? t('notificationsSettings.regenerating')
                    : t('notificationsSettings.regenerateKeys')}
                </button>
              </div>

              <div className={s.cardBody}>
                <div className={s.fieldGroup}>
                  <label htmlFor="vapid_public_key" className={s.label}>
                    <span>{t('notificationsSettings.publicKeyLabel')}</span>
                  </label>
                  <div className={s.fieldRow}>
                    <input
                      id="vapid_public_key"
                      name="vapid_public_key"
                      type="text"
                      autoComplete="off"
                      value={vapidPublicKey}
                      onInput={(e) => setVapidPublicKey((e.target as HTMLInputElement).value)}
                      placeholder={t('notificationsSettings.publicKeyPlaceholder')}
                      className={`${s.input} ${s.inputCode}`}
                    />
                    {vapidPublicKey && (
                      <button
                        type="button"
                        onClick={handleCopyKey}
                        className={s.actionBtn}
                        aria-label={copiedKey ? t('mediaServers.copied') : t('mediaServers.copyUrl')}
                      >
                        {copiedKey ? '✅' : t('mediaServers.copyUrl')}
                      </button>
                    )}
                  </div>
                </div>

                <div className={s.fieldGroup}>
                  <label htmlFor="vapid_subject" className={s.label}>
                    <span>{t('notificationsSettings.subjectLabel')}</span>
                  </label>
                  <input
                    id="vapid_subject"
                    name="vapid_subject"
                    type="text"
                    autoComplete="off"
                    value={vapidSubject}
                    onInput={(e) => setVapidSubject((e.target as HTMLInputElement).value)}
                    placeholder={t('notificationsSettings.subjectPlaceholder')}
                    className={s.input}
                  />
                </div>
              </div>
            </div>

            {/* 2. NOTIFICATION TRIGGERS */}
            {prefs && (
              <div className={s.section}>
                <div className={s.sectionHeader}>
                  <div className={s.sectionHeaderContent}>
                    <h2 className={s.sectionTitle}>
                      <span>⚡ {t('notificationsSettings.triggersTitle')}</span>
                    </h2>
                    <p className={s.sectionDesc}>{t('notificationsSettings.triggersDesc')}</p>
                  </div>
                </div>

                <div className={s.triggersList}>
                  {notifItems.map((notif) => (
                    <div key={notif.key} className={s.triggerItem}>
                      <div className={s.triggerInfo}>
                        <div className={s.triggerLabel}>{notif.label}</div>
                        <div className={s.triggerDesc}>{notif.description}</div>
                      </div>
                      <button
                        type="button"
                        role="switch"
                        aria-checked={Boolean(prefs[notif.key])}
                        className={prefs[notif.key] ? s.toggleOn : s.toggleOff}
                        onClick={() => void toggle(notif.key)}
                        disabled={saving}
                        aria-label={notif.label}
                      >
                        <span className={s.toggleKnob} />
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </PullToRefresh>
  )
}
