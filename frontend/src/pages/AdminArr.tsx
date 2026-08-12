import { useState, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import s from './AdminArr.module.css'

interface ArrSettings {
  radarr_url?: string
  radarr_api_key?: string
  sonarr_url?: string
  sonarr_api_key?: string
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

export function AdminArr({ path }: { path?: string }) {
  const [settings, setSettings] = useState<ArrSettings | null>(null)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)
  
  const [radarrRootFolders, setRadarrRootFolders] = useState<RootFolder[]>([])
  const [radarrQualityProfiles, setRadarrQualityProfiles] = useState<QualityProfile[]>([])
  const [sonarrRootFolders, setSonarrRootFolders] = useState<RootFolder[]>([])
  const [sonarrQualityProfiles, setSonarrQualityProfiles] = useState<QualityProfile[]>([])

  const fetchOptions = () => {
    apiFetch<RootFolder[]>('/arr/radarr/rootfolder').then(data => setRadarrRootFolders(data)).catch(err => console.error('Failed to load Radarr root folders:', err))
    apiFetch<QualityProfile[]>('/arr/radarr/qualityprofile').then(data => setRadarrQualityProfiles(data)).catch(err => console.error('Failed to load Radarr quality profiles:', err))
    apiFetch<RootFolder[]>('/arr/sonarr/rootfolder').then(data => setSonarrRootFolders(data)).catch(err => console.error('Failed to load Sonarr root folders:', err))
    apiFetch<QualityProfile[]>('/arr/sonarr/qualityprofile').then(data => setSonarrQualityProfiles(data)).catch(err => console.error('Failed to load Sonarr quality profiles:', err))
  }

  useEffect(() => {
    // Fetch settings
    apiFetch<ArrSettings>('/admin/arr').then(data => setSettings(data)).catch(err => setError(err.message))
    fetchOptions()
  }, [])

  const handleSave = async () => {
    if (!settings) return
    setSaving(true)
    setSaved(false)
    setError(null)
    try {
      await apiFetch('/admin/arr', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings)
      })
      setSaved(true)
      fetchOptions()
      setTimeout(() => setSaved(false), 3000)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setSaving(false)
    }
  }

  const updateSetting = (key: keyof ArrSettings, value: string) => {
    if (settings) {
      setSettings({ ...settings, [key]: value })
    }
  }

  const renderSection = (title: string, desc: string, prefix: 'radarr_std' | 'radarr_anime' | 'sonarr_std' | 'sonarr_anime') => {
    if (!settings) return null
    
    const isRadarr = prefix.startsWith('radarr')
    const rootFolders = isRadarr ? radarrRootFolders : sonarrRootFolders
    const qualityProfiles = isRadarr ? radarrQualityProfiles : sonarrQualityProfiles

    return (
      <div className={s.section}>
        <div className={s.sectionHeader}>
          <h2 className={s.sectionTitle}>{title}</h2>
          <p className={s.sectionDesc}>{desc}</p>
        </div>
        
        <label className={s.settingRow}>
          <span className={s.settingLabel}>Monitored</span>
          <select 
            className={s.select}
            value={settings[`${prefix}_monitored` as keyof ArrSettings] || 'true'}
            onChange={e => updateSetting(`${prefix}_monitored` as keyof ArrSettings, (e.target as HTMLSelectElement).value)}
          >
            <option value="true">Yes</option>
            <option value="false">No</option>
          </select>
        </label>
        
        <label className={s.settingRow}>
          <span className={s.settingLabel}>Search on Add</span>
          <select 
            className={s.select}
            value={settings[`${prefix}_search` as keyof ArrSettings] || 'false'}
            onChange={e => updateSetting(`${prefix}_search` as keyof ArrSettings, (e.target as HTMLSelectElement).value)}
          >
            <option value="true">Yes</option>
            <option value="false">No</option>
          </select>
        </label>

        <label className={s.settingRow}>
          <span className={s.settingLabel}>Root Folder</span>
          <select 
            className={s.select}
            value={settings[`${prefix}_root_folder` as keyof ArrSettings] || ''}
            onChange={e => updateSetting(`${prefix}_root_folder` as keyof ArrSettings, (e.target as HTMLSelectElement).value)}
          >
            <option value="">Select Root Folder...</option>
            {rootFolders.map(rf => (
              <option key={rf.id} value={rf.path}>{rf.path}</option>
            ))}
          </select>
        </label>

        <label className={s.settingRow}>
          <span className={s.settingLabel}>Quality Profile</span>
          <select 
            className={s.select}
            value={settings[`${prefix}_quality_profile` as keyof ArrSettings] || ''}
            onChange={e => updateSetting(`${prefix}_quality_profile` as keyof ArrSettings, (e.target as HTMLSelectElement).value)}
          >
            <option value="">Select Quality Profile...</option>
            {qualityProfiles.map(qp => (
              <option key={qp.id} value={qp.id}>{qp.name}</option>
            ))}
          </select>
        </label>
      </div>
    )
  }


  if (!settings && !error) return <div className={s.page}>Loading...</div>

  return (
    <div className={s.page}>
      <h1 className={s.title}>
        <button className={s.backBtn} onClick={() => route('/admin')} title="Back to Admin">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
        Radarr / Sonarr Defaults
      </h1>
      
      {error && <div style={{ color: 'var(--status-crit)', marginBottom: '1rem' }}>{error}</div>}

      {settings && (
        <div className={s.section}>
          <div className={s.sectionHeader}>
            <h2 className={s.sectionTitle}>Server Connections</h2>
            <p className={s.sectionDesc}>Configure Radarr & Sonarr server URLs and API keys (overrides environment variables if set)</p>
          </div>
          
          <label className={s.settingRow}>
            <span className={s.settingLabel}>Radarr URL</span>
            <input 
              type="text" 
              className={s.input} 
              placeholder="http://radarr:7878"
              value={settings.radarr_url || ''} 
              onChange={e => updateSetting('radarr_url', (e.target as HTMLInputElement).value)} 
            />
          </label>
          
          <label className={s.settingRow}>
            <span className={s.settingLabel}>Radarr API Key</span>
            <input 
              type="password" 
              className={s.input} 
              placeholder="Radarr API Key"
              value={settings.radarr_api_key || ''} 
              onChange={e => updateSetting('radarr_api_key', (e.target as HTMLInputElement).value)} 
            />
          </label>

          <label className={s.settingRow}>
            <span className={s.settingLabel}>Sonarr URL</span>
            <input 
              type="text" 
              className={s.input} 
              placeholder="http://sonarr:8989"
              value={settings.sonarr_url || ''} 
              onChange={e => updateSetting('sonarr_url', (e.target as HTMLInputElement).value)} 
            />
          </label>

          <label className={s.settingRow}>
            <span className={s.settingLabel}>Sonarr API Key</span>
            <input 
              type="password" 
              className={s.input} 
              placeholder="Sonarr API Key"
              value={settings.sonarr_api_key || ''} 
              onChange={e => updateSetting('sonarr_api_key', (e.target as HTMLInputElement).value)} 
            />
          </label>
        </div>
      )}

      {renderSection('Radarr (Standard)', 'Defaults for movies', 'radarr_std')}
      {renderSection('Radarr (Anime)', 'Defaults for anime movies', 'radarr_anime')}
      {renderSection('Sonarr (Standard)', 'Defaults for TV shows', 'sonarr_std')}
      {renderSection('Sonarr (Anime)', 'Defaults for anime shows', 'sonarr_anime')}

      <div className={s.bottomPad} />
      
      <div className={s.actionDrawer}>
        <button className={s.btnPrimary} onClick={handleSave} disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </button>
        {saved && <div className={s.successMsg}>Settings saved!</div>}
      </div>
    </div>
  )
}
