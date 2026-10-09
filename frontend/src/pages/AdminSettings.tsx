import type { JSX } from 'preact'
import { useState, useEffect } from 'preact/hooks'
import { apiFetch } from '../api'
import { AdminHeader } from '../components/AdminHeader'
import { THEMES, getStoredTheme, applyTheme, ThemeId } from '../utils/theme'
import { useTranslation, Locale } from '../i18n'
import { setPreferredMetadataLanguage } from '../utils'
import { ALL_WATCH_PROVIDERS, DEFAULT_ENABLED_PROVIDERS, setEnabledWatchProviders } from '../utils/providers'
import s from './AdminSettings.module.css'

export const METADATA_LANGUAGES = [
  { id: 'fr', flag: '🇫🇷', name: 'French', nativeName: 'Français' }, // i18n-ignore
  { id: 'en', flag: '🇬🇧', name: 'English', nativeName: 'English' },
  { id: 'de', flag: '🇩🇪', name: 'German', nativeName: 'Deutsch' },
  { id: 'es', flag: '🇪🇸', name: 'Spanish', nativeName: 'Español' }, // i18n-ignore
  { id: 'it', flag: '🇮🇹', name: 'Italian', nativeName: 'Italiano' },
  { id: 'pt', flag: '🇵🇹', name: 'Portuguese', nativeName: 'Português' }, // i18n-ignore
  { id: 'ja', flag: '🇯🇵', name: 'Japanese', nativeName: '日本語' },
] as const

interface SystemSettings {
  tmdb_api_key?: string
  tmdb_configured?: boolean
  tvdb_api_key?: string
  tvdb_configured?: boolean
  gemini_api_keys?: string
  gemini_configured?: boolean
  anilist_client_id?: string
  anilist_client_secret?: string
  anilist_configured?: boolean
  metadata_language?: string
  enabled_watch_providers?: string
}

export function AdminSettings({ path }: { path?: string }): JSX.Element {
  const { t, locale, setLocale, locales } = useTranslation()
  const [settings, setSettings] = useState<SystemSettings | null>(null)
  const [formValues, setFormValues] = useState<Record<string, string>>({})
  const [saving, setSaving] = useState(false)
  const [successMsg, setSuccessMsg] = useState<string | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)

  // Test states
  const [testResults, setTestResults] = useState<Record<string, { ok?: boolean; message?: string; error?: string; loading?: boolean }>>({})
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
        metadata_language: data.metadata_language || 'fr',
        enabled_watch_providers: data.enabled_watch_providers !== undefined ? data.enabled_watch_providers : DEFAULT_ENABLED_PROVIDERS,
      })
    } catch (err: unknown) {
      setErrorMsg(err instanceof Error ? err.message : 'Failed to load configuration')
    }
  }

  useEffect(() => {
    void loadSettings()
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
      if (formValues.metadata_language) {
        setPreferredMetadataLanguage(formValues.metadata_language)
      }
      if (formValues.enabled_watch_providers !== undefined) {
        setEnabledWatchProviders(formValues.enabled_watch_providers)
      }
      setSuccessMsg(t('arrSettings.savedSuccess'))
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

  if (!settings) {
    return (
      <div className={s.page}>
        <AdminHeader title={t('admin.systemSettings')} />
        <div style={{ padding: 'var(--space-xl)', textAlign: 'center', color: 'var(--ink-dim)' }}>
          {t('common.loading')}
        </div>
      </div>
    )
  }

  return (
    <div className={s.page}>
      <form onSubmit={handleSave} className={s.sectionsList}>
        <AdminHeader title={t('admin.systemSettings')}>
          <button type="submit" disabled={saving} className={s.saveBtn}>
            {saving ? t('arrSettings.saving') : t('arrSettings.saveSettings')}
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
                    onClick={() => setLocale(loc.id as Locale)}
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
              <span>🎬 {t('settings.metadataAi')}</span>
            </div>
          </div>
          <div className={s.sectionDesc}>
            {t('settings.metadataAiKeysDesc')}
          </div>

          {/* Primary Metadata Language */}
          <div className={s.fieldGroup} style={{ marginBottom: '24px', paddingBottom: '16px', borderBottom: '1px solid var(--border)' }}>
            <label className={s.label} style={{ marginBottom: '4px' }}>
              <span>🌐 {t('settings.metadataLanguage')}</span>
            </label>
            <div className={s.sectionDesc} style={{ marginBottom: '12px', fontSize: '13px' }}>
              {t('settings.metadataLanguageDesc')}
            </div>
            <div className={s.themeGrid}>
              {METADATA_LANGUAGES.map((lang) => {
                const isActive = (formValues.metadata_language || settings.metadata_language || 'fr') === lang.id
                return (
                  <button
                    key={lang.id}
                    type="button"
                    onClick={() => handleChange('metadata_language', lang.id)}
                    className={`${s.themeCard} ${isActive ? s.themeCardActive : ''}`}
                  >
                    <span style={{ fontSize: '20px', lineHeight: 1, flexShrink: 0 }}>
                      {lang.flag}
                    </span>
                    <div className={s.themeInfo}>
                      <span className={s.themeName}>{lang.nativeName}</span>
                      <span className={s.themeDesc}>{lang.name}</span>
                    </div>
                  </button>
                )
              })}
            </div>
          </div>

          {/* TMDB */}
          <div className={s.fieldGroup}>
            <label htmlFor="tmdb_api_key" className={s.label}>
              <span>TMDB API Key (TheMovieDB)</span>
              <span className={`${s.statusBadge} ${settings.tmdb_configured ? s.statusOk : s.statusMissing}`}>
                {settings.tmdb_configured ? t('settings.configured') : t('settings.notConfigured')}
              </span>
            </label>
            <div className={s.fieldRow}>
              <input
                id="tmdb_api_key"
                name="tmdb_api_key"
                type="text"
                autoComplete="off"
                value={formValues.tmdb_api_key}
                onInput={(e) => handleChange('tmdb_api_key', (e.target as HTMLInputElement).value)}
                placeholder={t('settings.tmdbPlaceholder')}
                className={`${s.input} ${s.inputCode}`}
              />
              <button
                type="button"
                disabled={testResults.tmdb?.loading}
                onClick={() => handleTest('tmdb', '/admin/system-settings/test/tmdb', { api_key: formValues.tmdb_api_key })}
                className={s.testBtn}
              >
                {testResults.tmdb?.loading ? t('settings.testing') : t('settings.test')}
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
                {settings.tvdb_configured ? t('settings.configured') : t('settings.notConfigured')}
              </span>
            </label>
            <div className={s.fieldRow}>
              <input
                id="tvdb_api_key"
                name="tvdb_api_key"
                type="text"
                autoComplete="off"
                value={formValues.tvdb_api_key}
                onInput={(e) => handleChange('tvdb_api_key', (e.target as HTMLInputElement).value)}
                placeholder={t('settings.tvdbPlaceholder')}
                className={`${s.input} ${s.inputCode}`}
              />
              <button
                type="button"
                disabled={testResults.tvdb?.loading}
                onClick={() => handleTest('tvdb', '/admin/system-settings/test/tvdb', { api_key: formValues.tvdb_api_key })}
                className={s.testBtn}
              >
                {testResults.tvdb?.loading ? t('settings.testing') : t('settings.test')}
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
                {settings.gemini_configured ? t('settings.configured') : t('settings.optional')}
              </span>
            </label>
            <div className={s.fieldRow}>
              <input
                id="gemini_api_keys"
                name="gemini_api_keys"
                type="text"
                autoComplete="off"
                value={formValues.gemini_api_keys}
                onInput={(e) => handleChange('gemini_api_keys', (e.target as HTMLInputElement).value)}
                placeholder={t('settings.geminiPlaceholder')}
                className={`${s.input} ${s.inputCode}`}
              />
              <button
                type="button"
                disabled={testResults.gemini?.loading}
                onClick={() => handleTest('gemini', '/admin/system-settings/test/gemini', { api_keys: formValues.gemini_api_keys })}
                className={s.testBtn}
              >
                {testResults.gemini?.loading ? t('settings.testing') : t('settings.test')}
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
                {settings.anilist_configured ? t('settings.configured') : t('settings.optional')}
              </span>
            </div>
            <div style={{ display: 'flex', gap: '8px' }}>
              <input
                id="anilist_client_id"
                name="anilist_client_id"
                type="text"
                autoComplete="off"
                value={formValues.anilist_client_id}
                onInput={(e) => handleChange('anilist_client_id', (e.target as HTMLInputElement).value)}
                placeholder={t('settings.clientIdPlaceholder')}
                className={`${s.input} ${s.inputCode}`}
                style={{ flex: 1 }}
              />
              <input
                id="anilist_client_secret"
                name="anilist_client_secret"
                type="text"
                autoComplete="off"
                value={formValues.anilist_client_secret}
                onInput={(e) => handleChange('anilist_client_secret', (e.target as HTMLInputElement).value)}
                placeholder={t('settings.clientSecretPlaceholder')}
                className={`${s.input} ${s.inputCode}`}
                style={{ flex: 1 }}
              />
            </div>
          </div>
        </div>

        {/* 2. STREAMING PLATFORMS & WATCH PROVIDERS */}
        <div className={s.sectionCard}>
          <div className={s.sectionHeader}>
            <div className={s.sectionTitle}>
              <span>📺 {t('settings.watchProviders')}</span>
            </div>
          </div>
          <div className={s.sectionDesc}>
            {t('settings.watchProvidersDesc')}
          </div>
          <div className={s.themeGrid}>
            {ALL_WATCH_PROVIDERS.map((provider) => {
              const currentEnabled = new Set(
                (formValues.enabled_watch_providers ?? DEFAULT_ENABLED_PROVIDERS)
                  .split(',')
                  .map((x) => x.trim())
                  .filter(Boolean)
              )
              const isEnabled = currentEnabled.has(provider.id)
              const toggle = () => {
                const next = new Set(currentEnabled)
                if (isEnabled) {
                  next.delete(provider.id)
                } else {
                  next.add(provider.id)
                }
                handleChange('enabled_watch_providers', Array.from(next).join(','))
              }
              return (
                <button
                  key={provider.id}
                  type="button"
                  onClick={toggle}
                  className={`${s.themeCard} ${isEnabled ? s.themeCardActive : ''}`}
                  style={{ cursor: 'pointer' }}
                >
                  <span
                    style={{
                      background: provider.bg,
                      color: provider.color,
                      border: provider.border ?? 'none',
                      fontSize: '10px',
                      fontWeight: 700,
                      padding: '2px 6px',
                      borderRadius: '4px',
                      textTransform: 'lowercase',
                      letterSpacing: '0.02em',
                      lineHeight: 1.2,
                      flexShrink: 0,
                    }}
                  >
                    {provider.shortName}
                  </span>
                  <div className={s.themeInfo}>
                    <span className={s.themeName}>{provider.name}</span>
                    <span className={s.themeDesc}>{isEnabled ? t('settings.providerActive') : t('settings.providerDisabled')}</span>
                  </div>
                </button>
              )
            })}
          </div>
        </div>
      </form>
    </div>
  )
}
