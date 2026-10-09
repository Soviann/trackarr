import type { JSX } from 'preact'
import { useState, useEffect } from 'preact/hooks'
import clsx from 'clsx'
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

function ChevronIcon({ isOpen }: { isOpen: boolean }) {
  return (
    <svg
      className={clsx(s.chevronIcon, isOpen && s.chevronOpen)}
      viewBox="0 0 20 20"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z"
        clipRule="evenodd"
      />
    </svg>
  )
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
  const [openDropdown, setOpenDropdown] = useState<'theme' | 'locale' | 'metadataLanguage' | 'providers' | null>(null)

  const toggleDropdown = (key: 'theme' | 'locale' | 'metadataLanguage' | 'providers') => {
    setOpenDropdown((prev) => (prev === key ? null : key))
  }

  useEffect(() => {
    if (!openDropdown) return
    const handlePointerDown = (e: PointerEvent) => {
      const target = e.target as HTMLElement | null
      if (!target?.closest(`.${s.dropdownContainer}`)) {
        setOpenDropdown(null)
      }
    }
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setOpenDropdown(null)
      }
    }
    document.addEventListener('pointerdown', handlePointerDown)
    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('pointerdown', handlePointerDown)
      document.removeEventListener('keydown', handleKeyDown)
    }
  }, [openDropdown])

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

  const currentTheme = THEMES.find((theme) => theme.id === selectedTheme) || THEMES[0]
  const currentLocale = locales.find((loc) => loc.id === locale) || locales[0]
  const currentMetaLangId = formValues.metadata_language || settings.metadata_language || 'fr'
  const currentMetaLang = METADATA_LANGUAGES.find((lang) => lang.id === currentMetaLangId) || METADATA_LANGUAGES[0]

  const enabledProvidersList = (formValues.enabled_watch_providers !== undefined
    ? formValues.enabled_watch_providers
    : (settings.enabled_watch_providers !== undefined ? settings.enabled_watch_providers : DEFAULT_ENABLED_PROVIDERS)
  )
    .split(',')
    .map((x) => x.trim())
    .filter(Boolean)
  const enabledProvidersSet = new Set(enabledProvidersList)
  const selectedProviders = ALL_WATCH_PROVIDERS.filter((p) => enabledProvidersSet.has(p.id))

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

        {/* 1. APPEARANCE & LOCALIZATION */}
        <div className={s.sectionCard}>
          <div className={s.sectionHeader}>
            <div className={s.sectionTitle}>
              <span>🎨 {t('settings.appearance')}</span>
            </div>
          </div>
          <div className={s.sectionDesc}>
            {t('settings.appearanceDesc')}
          </div>

          {/* Theme Dropdown */}
          <div className={s.fieldGroup}>
            <label className={s.label}>
              <span>{t('settings.theme')}</span>
            </label>
            <div className={s.fieldDesc}>
              {t('settings.themeDesc')}
            </div>
            <div className={s.dropdownContainer}>
              <button
                type="button"
                onClick={() => toggleDropdown('theme')}
                className={clsx(s.dropdownTrigger, openDropdown === 'theme' && s.dropdownTriggerOpen)}
                aria-expanded={openDropdown === 'theme'}
                aria-haspopup="listbox"
                data-testid="theme-dropdown"
              >
                <div className={s.dropdownSelected}>
                  <span
                    className={s.themePreviewDot}
                    style={{ background: currentTheme.gradient }}
                  />
                  <span className={s.dropdownSelectedTitle}>{currentTheme.name}</span>
                  <span className={s.dropdownSelectedSub}>{currentTheme.description}</span>
                </div>
                <ChevronIcon isOpen={openDropdown === 'theme'} />
              </button>
              {openDropdown === 'theme' && (
                <div className={s.dropdownMenu} role="listbox">
                  {THEMES.map((theme) => {
                    const isSelected = selectedTheme === theme.id
                    return (
                      <button
                        key={theme.id}
                        type="button"
                        role="option"
                        aria-selected={isSelected}
                        onClick={() => {
                          handleSelectTheme(theme.id)
                          setOpenDropdown(null)
                        }}
                        className={clsx(s.dropdownItem, isSelected && s.dropdownItemActive)}
                      >
                        <div className={s.dropdownItemLeft}>
                          <span
                            className={s.themePreviewDot}
                            style={{ background: theme.gradient }}
                          />
                          <div className={s.dropdownItemInfo}>
                            <span className={s.dropdownItemTitle}>{theme.name}</span>
                            <span className={s.dropdownItemSub}>{theme.description}</span>
                          </div>
                        </div>
                        {isSelected && <span className={s.checkMark}>✓</span>}
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
          </div>

          {/* Interface Language Dropdown */}
          <div className={s.fieldGroup}>
            <label className={s.label}>
              <span>🌐 {t('settings.language')}</span>
            </label>
            <div className={s.fieldDesc}>
              {t('settings.languageDesc')}
            </div>
            <div className={s.dropdownContainer}>
              <button
                type="button"
                onClick={() => toggleDropdown('locale')}
                className={clsx(s.dropdownTrigger, openDropdown === 'locale' && s.dropdownTriggerOpen)}
                aria-expanded={openDropdown === 'locale'}
                aria-haspopup="listbox"
                data-testid="interface-language-dropdown"
              >
                <div className={s.dropdownSelected}>
                  <span className={s.flagIcon}>{currentLocale.flag}</span>
                  <span className={s.dropdownSelectedTitle}>{currentLocale.nativeName}</span>
                  <span className={s.dropdownSelectedSub}>{currentLocale.name}</span>
                </div>
                <ChevronIcon isOpen={openDropdown === 'locale'} />
              </button>
              {openDropdown === 'locale' && (
                <div className={s.dropdownMenu} role="listbox">
                  {locales.map((loc) => {
                    const isSelected = locale === loc.id
                    return (
                      <button
                        key={loc.id}
                        type="button"
                        role="option"
                        aria-selected={isSelected}
                        onClick={() => {
                          setLocale(loc.id as Locale)
                          setOpenDropdown(null)
                        }}
                        className={clsx(s.dropdownItem, isSelected && s.dropdownItemActive)}
                      >
                        <div className={s.dropdownItemLeft}>
                          <span className={s.flagIcon}>{loc.flag}</span>
                          <div className={s.dropdownItemInfo}>
                            <span className={s.dropdownItemTitle}>{loc.nativeName}</span>
                            <span className={s.dropdownItemSub}>{loc.name}</span>
                          </div>
                        </div>
                        {isSelected && <span className={s.checkMark}>✓</span>}
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
          </div>

          {/* Primary Metadata Language Dropdown */}
          <div className={s.fieldGroup}>
            <label className={s.label}>
              <span>🌐 {t('settings.metadataLanguage')}</span>
            </label>
            <div className={s.fieldDesc}>
              {t('settings.metadataLanguageDesc')}
            </div>
            <div className={s.dropdownContainer}>
              <button
                type="button"
                onClick={() => toggleDropdown('metadataLanguage')}
                className={clsx(s.dropdownTrigger, openDropdown === 'metadataLanguage' && s.dropdownTriggerOpen)}
                aria-expanded={openDropdown === 'metadataLanguage'}
                aria-haspopup="listbox"
                data-testid="metadata-language-dropdown"
              >
                <div className={s.dropdownSelected}>
                  <span className={s.flagIcon}>{currentMetaLang.flag}</span>
                  <span className={s.dropdownSelectedTitle}>{currentMetaLang.nativeName}</span>
                  <span className={s.dropdownSelectedSub}>{currentMetaLang.name}</span>
                </div>
                <ChevronIcon isOpen={openDropdown === 'metadataLanguage'} />
              </button>
              {openDropdown === 'metadataLanguage' && (
                <div className={s.dropdownMenu} role="listbox">
                  {METADATA_LANGUAGES.map((lang) => {
                    const isSelected = currentMetaLangId === lang.id
                    return (
                      <button
                        key={lang.id}
                        type="button"
                        role="option"
                        aria-selected={isSelected}
                        onClick={() => {
                          handleChange('metadata_language', lang.id)
                          setOpenDropdown(null)
                        }}
                        className={clsx(s.dropdownItem, isSelected && s.dropdownItemActive)}
                      >
                        <div className={s.dropdownItemLeft}>
                          <span className={s.flagIcon}>{lang.flag}</span>
                          <div className={s.dropdownItemInfo}>
                            <span className={s.dropdownItemTitle}>{lang.nativeName}</span>
                            <span className={s.dropdownItemSub}>{lang.name}</span>
                          </div>
                        </div>
                        {isSelected && <span className={s.checkMark}>✓</span>}
                      </button>
                    )
                  })}
                </div>
              )}
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

          <div className={s.fieldGroup}>
            <div className={s.dropdownContainer}>
              <button
                type="button"
                onClick={() => toggleDropdown('providers')}
                className={clsx(s.dropdownTrigger, openDropdown === 'providers' && s.dropdownTriggerOpen)}
                aria-expanded={openDropdown === 'providers'}
                aria-haspopup="listbox"
                aria-label={t('settings.watchProviders')}
                data-testid="watch-providers-dropdown"
              >
                <div className={s.dropdownSelectedBadges}>
                  {selectedProviders.length === 0 ? (
                    <span className={s.dropdownPlaceholder}>{t('settings.noPlatformsSelected')}</span>
                  ) : selectedProviders.length === ALL_WATCH_PROVIDERS.length ? (
                    <span className={s.dropdownSelectedTitle}>
                      {t('settings.allPlatformsSelected', { count: selectedProviders.length })}
                    </span>
                  ) : (
                    selectedProviders.map((p) => (
                      <span
                        key={p.id}
                        className={s.providerBadge}
                        style={{
                          background: p.bg,
                          color: p.color,
                          border: p.border ?? 'none',
                        }}
                      >
                        {p.shortName}
                      </span>
                    ))
                  )}
                </div>
                <ChevronIcon isOpen={openDropdown === 'providers'} />
              </button>
              {openDropdown === 'providers' && (
                <div className={s.dropdownMenu} role="listbox" aria-multiselectable="true">
                  <div className={s.dropdownHeader}>
                    <span className={s.dropdownCount}>
                      {t('settings.selectedCount', {
                        count: selectedProviders.length,
                        total: ALL_WATCH_PROVIDERS.length,
                      })}
                    </span>
                    <div className={s.dropdownActions}>
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          handleChange('enabled_watch_providers', ALL_WATCH_PROVIDERS.map((p) => p.id).join(','))
                        }}
                        className={s.dropdownActionBtn}
                      >
                        {t('settings.selectAll')}
                      </button>
                      <button
                        type="button"
                        onClick={(e) => {
                          e.stopPropagation()
                          handleChange('enabled_watch_providers', '')
                        }}
                        className={s.dropdownActionBtn}
                      >
                        {t('settings.deselectAll')}
                      </button>
                    </div>
                  </div>
                  {ALL_WATCH_PROVIDERS.map((provider) => {
                    const isEnabled = enabledProvidersSet.has(provider.id)
                    return (
                      <button
                        key={provider.id}
                        type="button"
                        role="option"
                        aria-selected={isEnabled}
                        onClick={(e) => {
                          e.stopPropagation()
                          const next = new Set(enabledProvidersSet)
                          if (isEnabled) {
                            next.delete(provider.id)
                          } else {
                            next.add(provider.id)
                          }
                          handleChange('enabled_watch_providers', Array.from(next).join(','))
                        }}
                        className={clsx(s.dropdownItem, isEnabled && s.dropdownItemActive)}
                      >
                        <div className={s.dropdownItemLeft}>
                          <span className={clsx(s.checkbox, isEnabled && s.checkboxChecked)}>
                            {isEnabled && '✓'}
                          </span>
                          <span
                            className={s.providerBadge}
                            style={{
                              background: provider.bg,
                              color: provider.color,
                              border: provider.border ?? 'none',
                            }}
                          >
                            {provider.shortName}
                          </span>
                          <span className={s.dropdownItemTitle}>{provider.name}</span>
                        </div>
                        <span className={clsx(s.providerStatusBadge, isEnabled ? s.providerStatusActive : s.providerStatusDisabled)}>
                          {isEnabled ? t('settings.providerActive') : t('settings.providerDisabled')}
                        </span>
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
          </div>
        </div>

        {/* 3. METADATA & AI KEYS */}
        <div className={s.sectionCard}>
          <div className={s.sectionHeader}>
            <div className={s.sectionTitle}>
              <span>🎬 {t('settings.metadataAi')}</span>
            </div>
          </div>
          <div className={s.sectionDesc}>
            {t('settings.metadataAiKeysDesc')}
          </div>

          {/* TMDB */}
          <div className={s.fieldGroup}>
            <label htmlFor="tmdb_api_key" className={s.label}>
              <span>TMDB API Key (TheMovieDB)</span>
              <span className={clsx(s.statusBadge, settings.tmdb_configured ? s.statusOk : s.statusMissing)}>
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
                className={clsx(s.input, s.inputCode)}
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
              <span className={clsx(s.statusBadge, settings.tvdb_configured ? s.statusOk : s.statusMissing)}>
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
                className={clsx(s.input, s.inputCode)}
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
              <span className={clsx(s.statusBadge, settings.gemini_configured ? s.statusOk : s.statusMissing)}>
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
                className={clsx(s.input, s.inputCode)}
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
              <span className={clsx(s.statusBadge, settings.anilist_configured ? s.statusOk : s.statusMissing)}>
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
                className={clsx(s.input, s.inputCode)}
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
                className={clsx(s.input, s.inputCode)}
                style={{ flex: 1 }}
              />
            </div>
          </div>
        </div>
      </form>
    </div>
  )
}
