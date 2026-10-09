import type { JSX } from 'preact'
import { useState, useEffect } from 'preact/hooks'
import { apiFetch } from '../api'
import { AdminHeader } from '../components/AdminHeader'
import { useTranslation } from '../i18n'
import s from './AdminArr.module.css'

interface ArrSettings {
  radarr_url?: string
  radarr_api_key?: string
  sonarr_url?: string
  sonarr_api_key?: string
  prowlarr_url?: string
  prowlarr_api_key?: string
  radarr_std_monitored: string
  radarr_std_search: string
  radarr_std_root_folder: string
  radarr_std_quality_profile: string
  radarr_anime_monitored: string
  radarr_anime_search: string
  radarr_anime_root_folder: string
  radarr_anime_quality_profile: string
  sonarr_std_monitored: string
  sonarr_std_search: string
  sonarr_std_root_folder: string
  sonarr_std_quality_profile: string
  sonarr_anime_monitored: string
  sonarr_anime_search: string
  sonarr_anime_root_folder: string
  sonarr_anime_quality_profile: string
}

interface RootFolder {
  id: number
  path: string
}

interface QualityProfile {
  id: number
  name: string
}

interface TestResult {
  ok?: boolean
  message?: string
  error?: string
  loading?: boolean
}

type ArrService = 'radarr' | 'sonarr' | 'prowlarr'
type ArrSectionPrefix = 'radarr_std' | 'radarr_anime' | 'sonarr_std' | 'sonarr_anime'

export function AdminArr({ path }: { path?: string }): JSX.Element {
  const { t } = useTranslation()
  const [settings, setSettings] = useState<ArrSettings | null>(null)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [radarrRootFolders, setRadarrRootFolders] = useState<RootFolder[]>([])
  const [radarrQualityProfiles, setRadarrQualityProfiles] = useState<QualityProfile[]>([])
  const [sonarrRootFolders, setSonarrRootFolders] = useState<RootFolder[]>([])
  const [sonarrQualityProfiles, setSonarrQualityProfiles] = useState<QualityProfile[]>([])

  const [testResults, setTestResults] = useState<Record<ArrService, TestResult>>({
    radarr: {},
    sonarr: {},
    prowlarr: {},
  })

  const fetchOptions = () => {
    apiFetch<RootFolder[]>('/arr/radarr/rootfolder')
      .then(data => setRadarrRootFolders(data))
      .catch(err => console.error('Failed to load Radarr root folders:', err))
    apiFetch<QualityProfile[]>('/arr/radarr/qualityprofile')
      .then(data => setRadarrQualityProfiles(data))
      .catch(err => console.error('Failed to load Radarr quality profiles:', err))
    apiFetch<RootFolder[]>('/arr/sonarr/rootfolder')
      .then(data => setSonarrRootFolders(data))
      .catch(err => console.error('Failed to load Sonarr root folders:', err))
    apiFetch<QualityProfile[]>('/arr/sonarr/qualityprofile')
      .then(data => setSonarrQualityProfiles(data))
      .catch(err => console.error('Failed to load Sonarr quality profiles:', err))
  }

  useEffect(() => {
    apiFetch<ArrSettings>('/admin/arr')
      .then(data => setSettings(data))
      .catch(err => setError(err instanceof Error ? err.message : 'Failed to load Arr settings'))
    fetchOptions()
  }, [])

  useEffect(() => {
    if (!saved) return
    const timer = setTimeout(() => setSaved(false), 3000)
    return () => clearTimeout(timer)
  }, [saved])

  const handleSave = async () => {
    if (!settings) return
    setSaving(true)
    setSaved(false)
    setError(null)
    try {
      await apiFetch('/admin/arr', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings),
      })
      setSaved(true)
      fetchOptions()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  const updateSetting = (key: keyof ArrSettings, value: string) => {
    if (settings) {
      setSettings(prev => prev ? { ...prev, [key]: value } : prev)
    }
  }

  const handleTest = async (
    service: ArrService,
    endpoint: string,
    payload: { url: string; api_key: string }
  ) => {
    setTestResults(prev => ({ ...prev, [service]: { loading: true } }))
    try {
      const res = await apiFetch<{ ok: boolean; message?: string; error?: string }>(endpoint, {
        method: 'POST',
        body: JSON.stringify(payload),
      })
      setTestResults(prev => ({
        ...prev,
        [service]: { ok: res.ok, message: res.message, error: res.error, loading: false },
      }))
      if (res.ok && (service === 'radarr' || service === 'sonarr')) {
        fetchOptions()
      }
    } catch (err: unknown) {
      setTestResults(prev => ({
        ...prev,
        [service]: {
          ok: false,
          error: err instanceof Error ? err.message : 'Test request failed',
          loading: false,
        },
      }))
    }
  }

  const renderSection = (
    title: string,
    desc: string,
    prefix: ArrSectionPrefix
  ) => {
    if (!settings) return null

    const isRadarr = prefix.startsWith('radarr')
    const rootFolders = isRadarr ? radarrRootFolders : sonarrRootFolders
    const qualityProfiles = isRadarr ? radarrQualityProfiles : sonarrQualityProfiles

    const monitoredId = `${prefix}_monitored`
    const searchId = `${prefix}_search`
    const rootFolderId = `${prefix}_root_folder`
    const qualityProfileId = `${prefix}_quality_profile`

    return (
      <div className={s.section}>
        <div className={s.sectionHeader}>
          <h2 className={s.sectionTitle}>{title}</h2>
          <p className={s.sectionDesc}>{desc}</p>
        </div>

        <div className={s.settingRow}>
          <div className={s.settingInfo}>
            <label htmlFor={monitoredId} className={s.settingLabel}>{t('arrSettings.monitored')}</label>
          </div>
          <select
            id={monitoredId}
            name={monitoredId}
            className={s.select}
            value={settings[`${prefix}_monitored` as keyof ArrSettings] || 'true'}
            onChange={e => updateSetting(`${prefix}_monitored` as keyof ArrSettings, (e.target as HTMLSelectElement).value)}
          >
            <option value="true">{t('arrSettings.yes')}</option>
            <option value="false">{t('arrSettings.no')}</option>
          </select>
        </div>

        <div className={s.settingRow}>
          <div className={s.settingInfo}>
            <label htmlFor={searchId} className={s.settingLabel}>{t('arrSettings.searchOnAdd')}</label>
          </div>
          <select
            id={searchId}
            name={searchId}
            className={s.select}
            value={settings[`${prefix}_search` as keyof ArrSettings] || 'false'}
            onChange={e => updateSetting(`${prefix}_search` as keyof ArrSettings, (e.target as HTMLSelectElement).value)}
          >
            <option value="true">{t('arrSettings.yes')}</option>
            <option value="false">{t('arrSettings.no')}</option>
          </select>
        </div>

        <div className={s.settingRow}>
          <div className={s.settingInfo}>
            <label htmlFor={rootFolderId} className={s.settingLabel}>{t('arrSettings.rootFolder')}</label>
          </div>
          <select
            id={rootFolderId}
            name={rootFolderId}
            className={s.select}
            value={settings[`${prefix}_root_folder` as keyof ArrSettings] || ''}
            onChange={e => updateSetting(`${prefix}_root_folder` as keyof ArrSettings, (e.target as HTMLSelectElement).value)}
          >
            <option value="">{t('arrSettings.selectRootFolder')}</option>
            {rootFolders.map(rf => (
              <option key={rf.id} value={rf.path}>{rf.path}</option>
            ))}
          </select>
        </div>

        <div className={s.settingRow}>
          <div className={s.settingInfo}>
            <label htmlFor={qualityProfileId} className={s.settingLabel}>{t('arrSettings.qualityProfile')}</label>
          </div>
          <select
            id={qualityProfileId}
            name={qualityProfileId}
            className={s.select}
            value={settings[`${prefix}_quality_profile` as keyof ArrSettings] || ''}
            onChange={e => updateSetting(`${prefix}_quality_profile` as keyof ArrSettings, (e.target as HTMLSelectElement).value)}
          >
            <option value="">{t('arrSettings.selectQualityProfile')}</option>
            {qualityProfiles.map(qp => (
              <option key={qp.id} value={String(qp.id)}>{qp.name}</option>
            ))}
          </select>
        </div>
      </div>
    )
  }

  if (!settings && !error) {
    return (
      <div className={s.page}>
        <AdminHeader title={t('arrSettings.title')} />
        <div className={s.loading}>{t('common.loading')}</div>
      </div>
    )
  }

  if (!settings && error) {
    return (
      <div className={s.page}>
        <AdminHeader title={t('arrSettings.title')} />
        <div className={s.alertError}>{error}</div>
      </div>
    )
  }

  const isRadarrConfigured = Boolean(settings?.radarr_url && settings?.radarr_api_key)
  const isSonarrConfigured = Boolean(settings?.sonarr_url && settings?.sonarr_api_key)
  const isProwlarrConfigured = Boolean(settings?.prowlarr_url && settings?.prowlarr_api_key)

  return (
    <div className={s.page}>
      <AdminHeader title={t('arrSettings.title')}>
        <button
          type="button"
          className={s.saveBtn}
          onClick={handleSave}
          disabled={saving}
        >
          {saving ? t('arrSettings.saving') : t('arrSettings.saveSettings')}
        </button>
      </AdminHeader>

      {saved && <div className={s.alertSuccess} role="status">{t('arrSettings.savedSuccess')}</div>}
      {error && <div className={s.alertError} role="alert">{error}</div>}

      {/* CONNECTIONS CARD */}
      <div className={s.section}>
        <div className={s.sectionHeader}>
          <h2 className={s.sectionTitle}>{t('arrSettings.connectionsTitle')}</h2>
          <p className={s.sectionDesc}>{t('arrSettings.connectionsDesc')}</p>
        </div>
        <div className={s.connectionBody}>
          {/* Radarr */}
          <div className={s.connGroup}>
            <div className={s.connLabelRow}>
              <span className={s.connLabel}>{t('arrSettings.radarrLabel')}</span>
              <span className={`${s.statusBadge} ${isRadarrConfigured ? s.statusOk : s.statusMissing}`}>
                {isRadarrConfigured ? t('arrSettings.configured') : t('arrSettings.optional')}
              </span>
            </div>
            <div className={s.connInputRow}>
              <input
                id="radarr_url"
                name="radarr_url"
                type="text"
                aria-label={`${t('arrSettings.radarrLabel')} URL`}
                placeholder="http://radarr:7878"
                value={settings?.radarr_url || ''}
                onInput={e => updateSetting('radarr_url', (e.target as HTMLInputElement).value)}
                className={s.inputUrl}
                autocomplete="off"
              />
              <input
                id="radarr_api_key"
                name="radarr_api_key"
                type="text"
                aria-label={`${t('arrSettings.radarrLabel')} API Key`}
                placeholder="API Key"
                value={settings?.radarr_api_key || ''}
                onInput={e => updateSetting('radarr_api_key', (e.target as HTMLInputElement).value)}
                className={`${s.inputKey} ${s.inputCode}`}
                autocomplete="off"
              />
              <button
                type="button"
                aria-label={`${t('arrSettings.test')} ${t('arrSettings.radarrLabel')}`}
                disabled={testResults.radarr?.loading}
                onClick={() => handleTest('radarr', '/admin/system-settings/test/radarr', {
                  url: settings?.radarr_url || '',
                  api_key: settings?.radarr_api_key || '',
                })}
                className={s.testBtn}
              >
                {testResults.radarr?.loading ? t('arrSettings.testing') : t('arrSettings.test')}
              </button>
            </div>
            {testResults.radarr.ok !== undefined && (
              <div role="status">
                {testResults.radarr.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.radarr.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.radarr.error}</span>
                )}
              </div>
            )}
          </div>

          {/* Sonarr */}
          <div className={s.connGroup}>
            <div className={s.connLabelRow}>
              <span className={s.connLabel}>{t('arrSettings.sonarrLabel')}</span>
              <span className={`${s.statusBadge} ${isSonarrConfigured ? s.statusOk : s.statusMissing}`}>
                {isSonarrConfigured ? t('arrSettings.configured') : t('arrSettings.optional')}
              </span>
            </div>
            <div className={s.connInputRow}>
              <input
                id="sonarr_url"
                name="sonarr_url"
                type="text"
                aria-label={`${t('arrSettings.sonarrLabel')} URL`}
                placeholder="http://sonarr:8989"
                value={settings?.sonarr_url || ''}
                onInput={e => updateSetting('sonarr_url', (e.target as HTMLInputElement).value)}
                className={s.inputUrl}
                autocomplete="off"
              />
              <input
                id="sonarr_api_key"
                name="sonarr_api_key"
                type="text"
                aria-label={`${t('arrSettings.sonarrLabel')} API Key`}
                placeholder="API Key"
                value={settings?.sonarr_api_key || ''}
                onInput={e => updateSetting('sonarr_api_key', (e.target as HTMLInputElement).value)}
                className={`${s.inputKey} ${s.inputCode}`}
                autocomplete="off"
              />
              <button
                type="button"
                aria-label={`${t('arrSettings.test')} ${t('arrSettings.sonarrLabel')}`}
                disabled={testResults.sonarr?.loading}
                onClick={() => handleTest('sonarr', '/admin/system-settings/test/sonarr', {
                  url: settings?.sonarr_url || '',
                  api_key: settings?.sonarr_api_key || '',
                })}
                className={s.testBtn}
              >
                {testResults.sonarr?.loading ? t('arrSettings.testing') : t('arrSettings.test')}
              </button>
            </div>
            {testResults.sonarr.ok !== undefined && (
              <div role="status">
                {testResults.sonarr.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.sonarr.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.sonarr.error}</span>
                )}
              </div>
            )}
          </div>

          {/* Prowlarr */}
          <div className={s.connGroup}>
            <div className={s.connLabelRow}>
              <span className={s.connLabel}>{t('arrSettings.prowlarrLabel')}</span>
              <span className={`${s.statusBadge} ${isProwlarrConfigured ? s.statusOk : s.statusMissing}`}>
                {isProwlarrConfigured ? t('arrSettings.configured') : t('arrSettings.optional')}
              </span>
            </div>
            <div className={s.connInputRow}>
              <input
                id="prowlarr_url"
                name="prowlarr_url"
                type="text"
                aria-label={`${t('arrSettings.prowlarrLabel')} URL`}
                placeholder="http://prowlarr:9696"
                value={settings?.prowlarr_url || ''}
                onInput={e => updateSetting('prowlarr_url', (e.target as HTMLInputElement).value)}
                className={s.inputUrl}
                autocomplete="off"
              />
              <input
                id="prowlarr_api_key"
                name="prowlarr_api_key"
                type="text"
                aria-label={`${t('arrSettings.prowlarrLabel')} API Key`}
                placeholder="API Key"
                value={settings?.prowlarr_api_key || ''}
                onInput={e => updateSetting('prowlarr_api_key', (e.target as HTMLInputElement).value)}
                className={`${s.inputKey} ${s.inputCode}`}
                autocomplete="off"
              />
              <button
                type="button"
                aria-label={`${t('arrSettings.test')} ${t('arrSettings.prowlarrLabel')}`}
                disabled={testResults.prowlarr?.loading}
                onClick={() => handleTest('prowlarr', '/admin/system-settings/test/prowlarr', {
                  url: settings?.prowlarr_url || '',
                  api_key: settings?.prowlarr_api_key || '',
                })}
                className={s.testBtn}
              >
                {testResults.prowlarr?.loading ? t('arrSettings.testing') : t('arrSettings.test')}
              </button>
            </div>
            {testResults.prowlarr.ok !== undefined && (
              <div role="status">
                {testResults.prowlarr.ok ? (
                  <span className={s.testResultOk}>✅ {testResults.prowlarr.message}</span>
                ) : (
                  <span className={s.testResultErr}>❌ {testResults.prowlarr.error}</span>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* TWO-COLUMN DESKTOP GRID FOR DEFAULTS */}
      <div className={s.defaultsGrid}>
        <div className={s.column}>
          {renderSection(t('arrSettings.radarrStdTitle'), t('arrSettings.radarrStdDesc'), 'radarr_std')}
          {renderSection(t('arrSettings.radarrAnimeTitle'), t('arrSettings.radarrAnimeDesc'), 'radarr_anime')}
        </div>
        <div className={s.column}>
          {renderSection(t('arrSettings.sonarrStdTitle'), t('arrSettings.sonarrStdDesc'), 'sonarr_std')}
          {renderSection(t('arrSettings.sonarrAnimeTitle'), t('arrSettings.sonarrAnimeDesc'), 'sonarr_anime')}
        </div>
      </div>
    </div>
  )
}
