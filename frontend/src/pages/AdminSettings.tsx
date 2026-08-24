import { useState, useEffect } from 'preact/hooks'
import { apiFetch } from '../api'
import { routeTo } from '../routes'
import { THEMES, getStoredTheme, applyTheme, ThemeId } from '../utils/theme'
import { useTranslation, LOCALES, Locale } from '../i18n'
import s from './AdminSettings.module.css'

interface SystemSettings {
  tmdb_api_key: string
  tmdb_configured: boolean
  tvdb_api_key: string
  tvdb_configured: boolean
  gemini_api_keys: string
  gemini_configured: boolean
  anilist_client_id: string
  anilist_client_secret: string
  anilist_configured: boolean
  jellyfin_webhook_secret: string
  jellyfin_webhook_url: string
  plex_webhook_secret: string
  plex_webhook_url: string
  radarr_url: string
  radarr_api_key: string
  radarr_configured: boolean
  sonarr_url: string
  sonarr_api_key: string
  sonarr_configured: boolean
  prowlarr_url: string
  prowlarr_api_key: string
  prowlarr_configured: boolean
  vapid_public_key: string
  vapid_subject: string
  vapid_configured: boolean
}

export function AdminSettings({ path }: { path?: string }) {
  const { t, locale, setLocale, locales } = useTranslation()
  const [settings, setSettings] = useState<SystemSettings | null>(null)
  const [formValues, setFormValues] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  // Test states
  const [testResults, setTestResults] = useState<Record<string, { ok?: boolean; message?: string; error?: string; loading?: boolean }>>({})
  const [copiedKey, setCopiedKey] = useState<string | null>(null)
  const [selectedTheme, setSelectedTheme] = useState<ThemeId>(getStoredTheme())

  const handleSelectTheme = (themeId: ThemeId) => {
    setSelectedTheme(themeId)
    applyTheme(themeId)
  }

  const loadSettings = async () => {
    try {
      const data = await apiFetch<SystemSettings>('/admin/system-settings')
      setSettings(data)
      setFormValues({
        tmdb_api_key: data.tmdb_api_key || '',
        tvdb_api_key: data.tvdb_api_key || '',
        gemini_api_keys: data.gemini_api_keys || '',
        anilist_client_id: data.anilist_client_id || '',
        anilist_client_secret: data.anilist_client_secret || '',
        jellyfin_webhook_secret: data.jellyfin_webhook_secret || '',
        plex_webhook_secret: data.plex_webhook_secret || '',
        radarr_url: data.radarr_url || '',
        radarr_api_key: data.radarr_api_key || '',
        sonarr_url: data.sonarr_url || '',
        sonarr_api_key: data.sonarr_api_key || '',
        prowlarr_url: data.prowlarr_url || '',
        prowlarr_api_key: data.prowlarr_api_key || '',
        vapid_public_key: data.vapid_public_key || '',
        vapid_subject: data.vapid_subject || '',
      })
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : 'Failed to load configuration')
    }
  }

  useEffect(() => {
    loadSettings()
  }, [])

  const handleChange = (key: string, value: string) => {
    setFormValues((prev) => ({ ...prev, [key]: value }))
  }

  const handleSave = async (e: Event) => {
    e.preventDefault()
    setSaving(true)
    setSuccessMsg(null)
    setErrorMsg(null)

    try {
      await apiFetch('/admin/system-settings', {
        method: 'PUT',
        body: JSON.stringify(formValues),
      })
      setSuccessMsg('Settings saved and hot-reloaded successfully!')
      await loadSettings()
      setTimeout(() => setSuccessMsg(null), 4000)
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : 'Error while saving settings')
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async (serviceName: string, endpoint: string, payload?: Record<string, string>) => {
    setTestResults((prev) => ({ ...prev, [serviceName]: { loading: true } }))
    try {
      const res = await apiFetch<{ ok: boolean; message?: string; error?: string; version?: string }>(endpoint, {
        method: 'POST',
        body: JSON.stringify(payload || {}),
      })
      setTestResults((prev) => ({ ...prev, [serviceName]: { ok: res.ok, message: res.message, error: res.error, loading: false } }))
    } catch (err: unknown) {
      setTestResults((prev) => ({
        ...prev,
        [serviceName]: {
          ok: false,
          error: err instanceof Error ? err.message : 'Test request failed',
          loading: false,
        },
      }))
    }
  }

  const [generatingVapid, setGeneratingVapid] = useState(false)

  const handleGenerateVAPID = async () => {
    setGeneratingVapid(true)
    setErrorMsg(null)
    try {
      const res = await apiFetch<{ ok: boolean; message?: string; vapid_public_key?: string; vapid_subject?: string }>('/admin/system-settings/vapid/generate', {
        method: 'POST',
        body: JSON.stringify({ subject: formValues.vapid_subject }),
      })
      if (res.vapid_public_key) {
        setFormValues((prev) => ({
          ...prev,
          vapid_public_key: res.vapid_public_key || '',
          vapid_subject: res.vapid_subject || prev.vapid_subject,
        }))
      }
      setSuccessMsg(res.message || 'New VAPID keys generated successfully!')
      await loadSettings()
      setTimeout(() => setSuccessMsg(null), 4000)
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : 'Failed to generate VAPID keys')
    } finally {
      setGeneratingVapid(false)
    }
  }

  const handleCopy = (key: string, text: string) => {
    if (!text) return
    navigator.clipboard.writeText(text)
    setCopiedKey(key)
    setTimeout(() => setCopiedKey(null), 3000)
  }

  if (!settings) {
    return (
      <div className={s.page}>
        <div className={s.pageTitle}>Loading settings...</div>
      </div>
    )
  }

  return (
    <div className={s.page}>
      <form onSubmit={handleSave}>
        {/* TOP BAR */}
        <div className={s.topBar}>
          <div>
            <a href={routeTo.admin()} className={s.backLink}>
              ← Administration
            </a>
            <h1 className={s.pageTitle}>System Settings & API Keys</h1>
          </div>
          <button type="submit" disabled={saving} className={s.saveBtn}>
            {saving ? 'Saving...' : 'Save Settings'}
          </button>
        </div>

        {successMsg && <div className={s.alertSuccess}>{successMsg}</div>}
        {errorMsg && <div className={s.alertError}>{errorMsg}</div>}

        {/* 0. APPEARANCE & THEMES */}
        <div className={s.sectionCard}>
          <div className={s.sectionHeader}>
            <div className={s.sectionTitle}>
              <span>🎨 {t('settings.appearance')}</span>
            </div>
          </div>
          <div className={s.sectionDesc}>
            {t('settings.appearanceDesc')}
          </div>
          <div className={s.themeGrid}>
            {THEMES.map((theme) => {
              const isActive = selectedTheme === theme.id
              return (
                <button
                  key={theme.id}
                  type="button"
                  onClick={() => handleSelectTheme(theme.id)}
                  className={`${s.themeCard} ${isActive ? s.themeCardActive : ''}`}
                >
                  <span
                    className={s.themePreviewDot}
                    style={{ background: theme.gradient }}
                  />
                  <div className={s.themeInfo}>
                    <span className={s.themeName}>{theme.name}</span>
                    <span className={s.themeDesc}>{theme.description}</span>
                  </div>
                </button>
              )
            })}
          </div>

          <div style={{ marginTop: '16px', paddingTop: '16px', borderTop: '1px solid var(--border)' }}>
            <div className={s.label} style={{ marginBottom: '8px' }}>
              <span>🌐 {t('settings.language')}</span>
            </div>
            <div className={s.themeGrid}>
              {locales.map((loc) => {
                const isActive = locale === loc.id
                return (
                  <button
                    key={loc.id}
                    type="button"
                    onClick={() => setLocale(loc.id)}
                    className={`${s.themeCard} ${isActive ? s.themeCardActive : ''}`}
                  >
                    <span style={{ fontSize: '20px', lineHeight: 1, flexShrink: 0 }}>
                      {loc.flag}
                    </span>
                    <div className={s.themeInfo}>
                      <span className={s.themeName}>{loc.nativeName}</span>
                      <span className={s.themeDesc}>{loc.name}</span>
                    </div>
                  </button>
                )
              })}
            </div>
          </div>
        </div>

        {/* 1. METADATA & AI */}
        <div className={s.sectionCard}>
          <div className={s.sectionHeader}>
            <div className={s.sectionTitle}>
              <span>🎬 Metadata & Artificial Intelligence</span>
            </div>
          </div>
          <div className={s.sectionDesc}>
            These keys enable search, automatic enrichment, and intelligent title reconciliation for movies, TV shows, and anime.
          </div>

          {/* TMDB */}
          <div className={s.fieldGroup}>
            <label htmlFor="tmdb_api_key" className={s.label}>
              <span>TMDB API Key (TheMovieDB)</span>
              <span className={`${s.statusBadge} ${settings.tmdb_configured ? s.statusOk : s.statusMissing}`}>
                {settings.tmdb_configured ? 'Configured' : 'Not configured'}
              </span>
            </label>
            <div className={s.fieldRow}>
              <input
                id="tmdb_api_key"
                name="tmdb_api_key"
                type="text"
                autocomplete="off"
                value={formValues.tmdb_api_key}
                onInput={(e) => handleChange('tmdb_api_key', (e.target as HTMLInputElement).value)}
                placeholder="v3 API Key (32 hex characters)"
                className={`${s.input} ${s.inputCode}`}
              />
              <button
                type="button"
                disabled={testResults.tmdb?.loading}
                onClick={() => handleTest('tmdb', '/admin/system-settings/test/tmdb', { api_key: formValues.tmdb_api_key })}
                className={s.testBtn}
              >
                {testResults.tmdb?.loading ? 'Testing...' : 'Test'}
              </button>
            </div>
            {testResults.tmdb && (
              <div>
                {testResults.tmdb.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.tmdb.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.tmdb.error}</span>
                )}
              </div>
            )}
          </div>

          {/* TVDB */}
          <div className={s.fieldGroup}>
            <label htmlFor="tvdb_api_key" className={s.label}>
              <span>TheTVDB API Key (v4)</span>
              <span className={`${s.statusBadge} ${settings.tvdb_configured ? s.statusOk : s.statusMissing}`}>
                {settings.tvdb_configured ? 'Configured' : 'Not configured'}
              </span>
            </label>
            <div className={s.fieldRow}>
              <input
                id="tvdb_api_key"
                name="tvdb_api_key"
                type="text"
                autocomplete="off"
                value={formValues.tvdb_api_key}
                onInput={(e) => handleChange('tvdb_api_key', (e.target as HTMLInputElement).value)}
                placeholder="TheTVDB API v4 Project Key"
                className={`${s.input} ${s.inputCode}`}
              />
              <button
                type="button"
                disabled={testResults.tvdb?.loading}
                onClick={() => handleTest('tvdb', '/admin/system-settings/test/tvdb', { api_key: formValues.tvdb_api_key })}
                className={s.testBtn}
              >
                {testResults.tvdb?.loading ? 'Testing...' : 'Test'}
              </button>
            </div>
            {testResults.tvdb && (
              <div>
                {testResults.tvdb.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.tvdb.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.tvdb.error}</span>
                )}
              </div>
            )}
          </div>

          {/* Gemini */}
          <div className={s.fieldGroup}>
            <label htmlFor="gemini_api_keys" className={s.label}>
              <span>Google Gemini API Key(s)</span>
              <span className={`${s.statusBadge} ${settings.gemini_configured ? s.statusOk : s.statusMissing}`}>
                {settings.gemini_configured ? 'Configured' : 'Optional'}
              </span>
            </label>
            <div className={s.fieldRow}>
              <input
                id="gemini_api_keys"
                name="gemini_api_keys"
                type="text"
                autocomplete="off"
                value={formValues.gemini_api_keys}
                onInput={(e) => handleChange('gemini_api_keys', (e.target as HTMLInputElement).value)}
                placeholder="API keys (comma-separated for automatic rotation)"
                className={`${s.input} ${s.inputCode}`}
              />
              <button
                type="button"
                disabled={testResults.gemini?.loading}
                onClick={() => handleTest('gemini', '/admin/system-settings/test/gemini', { api_keys: formValues.gemini_api_keys })}
                className={s.testBtn}
              >
                {testResults.gemini?.loading ? 'Testing...' : 'Test'}
              </button>
            </div>
            {testResults.gemini && (
              <div>
                {testResults.gemini.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.gemini.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.gemini.error}</span>
                )}
              </div>
            )}
          </div>

          {/* AniList OAuth Client */}
          <div className={s.fieldGroup}>
            <div className={s.label}>
              <span>AniList Client ID & Secret</span>
              <span className={`${s.statusBadge} ${settings.anilist_configured ? s.statusOk : s.statusMissing}`}>
                {settings.anilist_configured ? 'Configured' : 'Optional'}
              </span>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                id="anilist_client_id"
                name="anilist_client_id"
                type="text"
                autocomplete="off"
                value={formValues.anilist_client_id}
                onInput={(e) => handleChange('anilist_client_id', (e.target as HTMLInputElement).value)}
                placeholder="Client ID"
                className={`${s.input} ${s.inputCode}`}
                style={{ flex: 1 }}
              />
              <input
                id="anilist_client_secret"
                name="anilist_client_secret"
                type="text"
                autocomplete="off"
                value={formValues.anilist_client_secret}
                onInput={(e) => handleChange('anilist_client_secret', (e.target as HTMLInputElement).value)}
                placeholder="Client Secret"
                className={`${s.input} ${s.inputCode}`}
                style={{ flex: 1 }}
              />
            </div>
          </div>
        </div>

        {/* 2. MEDIA SERVERS & WEBHOOKS */}
        <div className={s.sectionCard}>
          <div className={s.sectionHeader}>
            <div className={s.sectionTitle}>
              <span>📺 Media Servers & Webhooks</span>
            </div>
          </div>
          <div className={s.sectionDesc}>
            These secret tokens secure your scrobble endpoints against unauthorized requests.
          </div>

          {/* Webhook Plex */}
          <div className={s.fieldGroup}>
            <label htmlFor="plex_webhook_secret" className={s.label}>Plex Webhook Secret</label>
            <div className={s.fieldRow}>
              <input
                id="plex_webhook_secret"
                name="plex_webhook_secret"
                type="text"
                autocomplete="off"
                value={formValues.plex_webhook_secret}
                onInput={(e) => handleChange('plex_webhook_secret', (e.target as HTMLInputElement).value)}
                placeholder="Secret token for Plex URL"
                className={`${s.input} ${s.inputCode}`}
              />
              {settings.plex_webhook_url && (
                <button
                  type="button"
                  onClick={() => handleCopy('plex', settings.plex_webhook_url)}
                  className={s.copyBtn}
                >
                  {copiedKey === 'plex' ? '✅ Copied!' : 'Copy URL'}
                </button>
              )}
            </div>
            {settings.plex_webhook_url && (
              <div className={s.webhookHelp}>
                <strong>Plex URL to paste in Settings &gt; Webhooks:</strong><br />
                <code>{settings.plex_webhook_url}</code>
              </div>
            )}
          </div>

          {/* Webhook Jellyfin */}
          <div className={s.fieldGroup}>
            <label htmlFor="jellyfin_webhook_secret" className={s.label}>Jellyfin Webhook Secret</label>
            <div className={s.fieldRow}>
              <input
                id="jellyfin_webhook_secret"
                name="jellyfin_webhook_secret"
                type="text"
                autocomplete="off"
                value={formValues.jellyfin_webhook_secret}
                onInput={(e) => handleChange('jellyfin_webhook_secret', (e.target as HTMLInputElement).value)}
                placeholder="Secret token for Jellyfin URL"
                className={`${s.input} ${s.inputCode}`}
              />
              {settings.jellyfin_webhook_url && (
                <button
                  type="button"
                  onClick={() => handleCopy('jellyfin', settings.jellyfin_webhook_url)}
                  className={s.copyBtn}
                >
                  {copiedKey === 'jellyfin' ? '✅ Copied!' : 'Copy URL'}
                </button>
              )}
            </div>
            {settings.jellyfin_webhook_url && (
              <div className={s.webhookHelp}>
                <strong>Jellyfin URL to paste in the Webhook plugin:</strong><br />
                <code>{settings.jellyfin_webhook_url}</code>
              </div>
            )}
          </div>
        </div>

        {/* 3. DOWNLOAD STACK (ARR) */}
        <div className={s.sectionCard}>
          <div className={s.sectionHeader}>
            <div className={s.sectionTitle}>
              <span>📦 Download Stack (Radarr / Sonarr / Prowlarr)</span>
            </div>
          </div>
          <div className={s.sectionDesc}>
            Connect your download managers to automatically send detected movies and TV shows to your download queue.
          </div>

          {/* Radarr */}
          <div className={s.fieldGroup}>
            <div className={s.label}>
              <span>Radarr (Movies)</span>
              <span className={`${s.statusBadge} ${settings.radarr_configured ? s.statusOk : s.statusMissing}`}>
                {settings.radarr_configured ? 'Configured' : 'Optional'}
              </span>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                id="radarr_url"
                name="radarr_url"
                type="text"
                autocomplete="off"
                value={formValues.radarr_url}
                onInput={(e) => handleChange('radarr_url', (e.target as HTMLInputElement).value)}
                placeholder="http://radarr:7878"
                className={s.input}
                style={{ flex: 1.2 }}
              />
              <input
                id="radarr_api_key"
                name="radarr_api_key"
                type="text"
                autocomplete="off"
                value={formValues.radarr_api_key}
                onInput={(e) => handleChange('radarr_api_key', (e.target as HTMLInputElement).value)}
                placeholder="API Key"
                className={`${s.input} ${s.inputCode}`}
                style={{ flex: 1 }}
              />
              <button
                type="button"
                disabled={testResults.radarr?.loading}
                onClick={() => handleTest('radarr', '/admin/system-settings/test/radarr', { url: formValues.radarr_url, api_key: formValues.radarr_api_key })}
                className={s.testBtn}
              >
                {testResults.radarr?.loading ? '...' : 'Test'}
              </button>
            </div>
            {testResults.radarr && (
              <div>
                {testResults.radarr.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.radarr.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.radarr.error}</span>
                )}
              </div>
            )}
          </div>

          {/* Sonarr */}
          <div className={s.fieldGroup}>
            <div className={s.label}>
              <span>Sonarr (TV & Anime)</span>
              <span className={`${s.statusBadge} ${settings.sonarr_configured ? s.statusOk : s.statusMissing}`}>
                {settings.sonarr_configured ? 'Configured' : 'Optional'}
              </span>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                id="sonarr_url"
                name="sonarr_url"
                type="text"
                autocomplete="off"
                value={formValues.sonarr_url}
                onInput={(e) => handleChange('sonarr_url', (e.target as HTMLInputElement).value)}
                placeholder="http://sonarr:8989"
                className={s.input}
                style={{ flex: 1.2 }}
              />
              <input
                id="sonarr_api_key"
                name="sonarr_api_key"
                type="text"
                autocomplete="off"
                value={formValues.sonarr_api_key}
                onInput={(e) => handleChange('sonarr_api_key', (e.target as HTMLInputElement).value)}
                placeholder="API Key"
                className={`${s.input} ${s.inputCode}`}
                style={{ flex: 1 }}
              />
              <button
                type="button"
                disabled={testResults.sonarr?.loading}
                onClick={() => handleTest('sonarr', '/admin/system-settings/test/sonarr', { url: formValues.sonarr_url, api_key: formValues.sonarr_api_key })}
                className={s.testBtn}
              >
                {testResults.sonarr?.loading ? '...' : 'Test'}
              </button>
            </div>
            {testResults.sonarr && (
              <div>
                {testResults.sonarr.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.sonarr.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.sonarr.error}</span>
                )}
              </div>
            )}
          </div>

          {/* Prowlarr */}
          <div className={s.fieldGroup}>
            <div className={s.label}>
              <span>Prowlarr (Release Indexers)</span>
              <span className={`${s.statusBadge} ${settings.prowlarr_configured ? s.statusOk : s.statusMissing}`}>
                {settings.prowlarr_configured ? 'Configured' : 'Optional'}
              </span>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                id="prowlarr_url"
                name="prowlarr_url"
                type="text"
                autocomplete="off"
                value={formValues.prowlarr_url}
                onInput={(e) => handleChange('prowlarr_url', (e.target as HTMLInputElement).value)}
                placeholder="http://prowlarr:9696"
                className={s.input}
                style={{ flex: 1.2 }}
              />
              <input
                id="prowlarr_api_key"
                name="prowlarr_api_key"
                type="text"
                autocomplete="off"
                value={formValues.prowlarr_api_key}
                onInput={(e) => handleChange('prowlarr_api_key', (e.target as HTMLInputElement).value)}
                placeholder="API Key"
                className={`${s.input} ${s.inputCode}`}
                style={{ flex: 1 }}
              />
              <button
                type="button"
                disabled={testResults.prowlarr?.loading}
                onClick={() => handleTest('prowlarr', '/admin/system-settings/test/prowlarr', { url: formValues.prowlarr_url, api_key: formValues.prowlarr_api_key })}
                className={s.testBtn}
              >
                {testResults.prowlarr?.loading ? '...' : 'Test'}
              </button>
            </div>
            {testResults.prowlarr && (
              <div>
                {testResults.prowlarr.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.prowlarr.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.prowlarr.error}</span>
                )}
              </div>
            )}
          </div>
        </div>

        {/* 4. WEB PUSH NOTIFICATIONS (VAPID) */}
        <div className={s.sectionCard}>
          <div className={s.sectionHeader}>
            <div className={s.sectionTitle}>
              <span>🔔 Web Push Notifications (VAPID)</span>
              <span className={`${s.statusBadge} ${settings.vapid_configured ? s.statusOk : s.statusMissing}`}>
                {settings.vapid_configured ? 'Auto-configured' : 'Not configured'}
              </span>
            </div>
            <button
              type="button"
              disabled={generatingVapid}
              onClick={handleGenerateVAPID}
              className={s.testBtn}
            >
              {generatingVapid ? 'Generating...' : '⚡ Regenerate Keys'}
            </button>
          </div>
          <div className={s.sectionDesc}>
            Managed automatically by Trackarr to deliver push notifications to mobile and browsers (HTTPS required in production).
          </div>

          <div className={s.fieldGroup}>
            <label htmlFor="vapid_public_key" className={s.label}>VAPID Public Key</label>
            <input
              id="vapid_public_key"
              name="vapid_public_key"
              type="text"
              autocomplete="off"
              value={formValues.vapid_public_key}
              onInput={(e) => handleChange('vapid_public_key', (e.target as HTMLInputElement).value)}
              placeholder="Base64 URL-safe Public Key (Generated automatically)"
              className={`${s.input} ${s.inputCode}`}
            />
          </div>

          <div className={s.fieldGroup}>
            <label htmlFor="vapid_subject" className={s.label}>Admin Contact Email (VAPID Subject)</label>
            <input
              id="vapid_subject"
              name="vapid_subject"
              type="text"
              autocomplete="off"
              value={formValues.vapid_subject}
              onInput={(e) => handleChange('vapid_subject', (e.target as HTMLInputElement).value)}
              placeholder="mailto:admin@example.com"
              className={s.input}
            />
          </div>
        </div>
      </form>
    </div>
  )
}
