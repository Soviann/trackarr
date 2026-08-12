import { useState, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import type { TitleType } from '../types'
import { CoverImage } from '../components/CoverImage'
import s from './ArrQueue.module.css'

interface QueueItem {
  id: number
  type: TitleType
  cover_url: string | null
  name: string
  is_anime: boolean
  year: number
  tmdb_id: number | null
  tvdb_id: number | null
}

interface RootFolder {
  id: number
  path: string
}

interface QualityProfile {
  id: number
  name: string
}

interface ArrSettings {
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

export function ArrQueue({ path }: { path?: string }) {
  const [queue, setQueue] = useState<QueueItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [settings, setSettings] = useState<ArrSettings | null>(null)
  
  const [radarrRootFolders, setRadarrRootFolders] = useState<RootFolder[]>([])
  const [radarrQualityProfiles, setRadarrQualityProfiles] = useState<QualityProfile[]>([])
  const [sonarrRootFolders, setSonarrRootFolders] = useState<RootFolder[]>([])
  const [sonarrQualityProfiles, setSonarrQualityProfiles] = useState<QualityProfile[]>([])

  // Store per-item selected values
  const [itemForms, setItemForms] = useState<Record<number, {
    monitored: string
    search: string
    rootFolder: string
    qualityProfile: string
  }>>({})
  
  const [pushing, setPushing] = useState<Record<number, boolean>>({})
  const [offset, setOffset] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)

  const processQueueItems = (queueData: QueueItem[], settingsData: ArrSettings) => {
    const forms: typeof itemForms = {}
    for (const item of queueData) {
      let prefix = ''
      if (item.type === 'movie') {
        prefix = item.is_anime ? 'radarr_anime' : 'radarr_std'
      } else {
        prefix = item.is_anime ? 'sonarr_anime' : 'sonarr_std'
      }
      
      forms[item.id] = {
        monitored: (settingsData as any)[`${prefix}_monitored`] || 'true',
        search: (settingsData as any)[`${prefix}_search`] || 'false',
        rootFolder: (settingsData as any)[`${prefix}_root_folder`] || '',
        qualityProfile: (settingsData as any)[`${prefix}_quality_profile`] || ''
      }
    }
    return forms
  }

  useEffect(() => {
    Promise.all([
      apiFetch<{items: QueueItem[], has_more: boolean}>('/arr/queue?limit=50&offset=0'),
      apiFetch<ArrSettings>('/admin/arr'),
      apiFetch<RootFolder[]>('/arr/radarr/rootfolder').catch(err => { console.error('Failed to load Radarr root folders:', err); return [] }),
      apiFetch<QualityProfile[]>('/arr/radarr/qualityprofile').catch(err => { console.error('Failed to load Radarr quality profiles:', err); return [] }),
      apiFetch<RootFolder[]>('/arr/sonarr/rootfolder').catch(err => { console.error('Failed to load Sonarr root folders:', err); return [] }),
      apiFetch<QualityProfile[]>('/arr/sonarr/qualityprofile').catch(err => { console.error('Failed to load Sonarr quality profiles:', err); return [] })
    ]).then(([queueRes, settingsData, rr, rq, sr, sq]) => {
      const queueData = queueRes.items || []
      setQueue(queueData)
      setHasMore(queueRes.has_more)
      setSettings(settingsData)
      setRadarrRootFolders(rr)
      setRadarrQualityProfiles(rq)
      setSonarrRootFolders(sr)
      setSonarrQualityProfiles(sq)
      
      setItemForms(processQueueItems(queueData, settingsData))
      setLoading(false)
    }).catch(err => {
      setError(err.message)
      setLoading(false)
    })
  }, [])

  const loadMore = async () => {
    if (loadingMore || !hasMore || !settings) return
    setLoadingMore(true)
    const nextOffset = queue.length
    try {
      const res = await apiFetch<{items: QueueItem[], has_more: boolean}>(`/arr/queue?limit=50&offset=${nextOffset}`)
      const newItems = res.items || []
      setQueue(prev => [...prev, ...newItems])
      setHasMore(res.has_more)
      setOffset(nextOffset)
      setItemForms(prev => ({ ...prev, ...processQueueItems(newItems, settings) }))
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoadingMore(false)
    }
  }

  const updateForm = (id: number, field: string, value: string) => {
    setItemForms(prev => ({
      ...prev,
      [id]: { ...prev[id], [field]: value }
    }))
  }

  const handlePush = async (item: QueueItem) => {
    const form = itemForms[item.id]
    if (!form || !form.rootFolder || !form.qualityProfile) {
      alert('Please select root folder and quality profile.')
      return
    }
    
    setPushing(prev => ({ ...prev, [item.id]: true }))
    try {
      await apiFetch(`/arr/queue/${item.id}/push`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          monitored: form.monitored === 'true',
          search: form.search === 'true',
          root_folder: form.rootFolder,
          quality_profile: parseInt(form.qualityProfile, 10)
        })
      })
      // Remove from queue
      setQueue(prev => prev.filter(q => q.id !== item.id))
    } catch (err: any) {
      alert(`Push failed: ${err.message}`)
    } finally {
      setPushing(prev => ({ ...prev, [item.id]: false }))
    }
  }

  const handleIgnore = async (item: QueueItem) => {
    try {
      await apiFetch(`/titles/${item.id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ arr_ignored: true })
      })
      setQueue(prev => prev.filter(q => q.id !== item.id))
    } catch (err: any) {
      alert(`Ignore failed: ${err.message}`)
    }
  }

  if (loading) return <div className={s.page}>Loading...</div>
  if (error) return <div className={s.page}>Error: {error}</div>

  return (
    <div className={s.page}>
      <div className={s.header}>
        <button className={s.backBtn} onClick={() => route('/admin')} title="Back to Admin">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
        <h1 className={s.title}>Arr Review Queue</h1>
      </div>

      {queue.length === 0 ? (
        <div className={s.empty}>The queue is empty.</div>
      ) : (
        queue.map(item => {
          const isRadarr = item.type === 'movie'
          const rootFolders = isRadarr ? radarrRootFolders : sonarrRootFolders
          const qualityProfiles = isRadarr ? radarrQualityProfiles : sonarrQualityProfiles
          const form = itemForms[item.id]

          return (
            <div key={item.id} className={s.card}>
              <CoverImage
                coverUrl={item.cover_url}
                type={item.type}
                is_anime={item.is_anime}
                alt={item.name}
                className={`${s.poster} ${s.clickable}`}
                iconSize="28px"
                onClick={() => route(`/title/${item.id}`)}
              />
              
              <div className={s.info}>
                <h3 className={`${s.itemTitle} ${s.clickable}`} onClick={() => route(`/title/${item.id}`)}>{item.name}</h3>
                <div className={s.meta}>
                  <span>{item.type === 'movie' ? 'Movie' : 'TV Show'}</span>
                  {item.year && <span>• {item.year}</span>}
                  {item.is_anime && <span>• Anime</span>}
                </div>
                
                {form && (
                  <div className={s.formGrid}>
                    <div className={s.formGroup}>
                      <span className={s.formLabel}>Monitored</span>
                      <select className={s.select} value={form.monitored} onChange={e => updateForm(item.id, 'monitored', (e.target as HTMLSelectElement).value)}>
                        <option value="true">Yes</option>
                        <option value="false">No</option>
                      </select>
                    </div>
                    
                    <div className={s.formGroup}>
                      <span className={s.formLabel}>Search on Add</span>
                      <select className={s.select} value={form.search} onChange={e => updateForm(item.id, 'search', (e.target as HTMLSelectElement).value)}>
                        <option value="true">Yes</option>
                        <option value="false">No</option>
                      </select>
                    </div>
                    
                    <div className={s.formGroup}>
                      <span className={s.formLabel}>Root Folder</span>
                      <select className={s.select} value={form.rootFolder} onChange={e => updateForm(item.id, 'rootFolder', (e.target as HTMLSelectElement).value)}>
                        <option value="">Select...</option>
                        {rootFolders.map(rf => <option key={rf.id} value={rf.path}>{rf.path}</option>)}
                      </select>
                    </div>
                    
                    <div className={s.formGroup}>
                      <span className={s.formLabel}>Quality Profile</span>
                      <select className={s.select} value={form.qualityProfile} onChange={e => updateForm(item.id, 'qualityProfile', (e.target as HTMLSelectElement).value)}>
                        <option value="">Select...</option>
                        {qualityProfiles.map(qp => <option key={qp.id} value={qp.id}>{qp.name}</option>)}
                      </select>
                    </div>
                  </div>
                )}
                
                <div className={s.actions}>
                  <button className={s.ignoreBtn} onClick={() => handleIgnore(item)}>
                    Ignore
                  </button>
                  <button className={s.pushBtn} onClick={() => handlePush(item)} disabled={pushing[item.id]}>
                    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                      <line x1="22" y1="2" x2="11" y2="13" />
                      <polygon points="22 2 15 22 11 13 2 9 22 2" />
                    </svg>
                    {pushing[item.id] ? 'Pushing...' : 'Push to ' + (isRadarr ? 'Radarr' : 'Sonarr')}
                  </button>
                </div>
              </div>
            </div>
          )
        })
      )}
      
      {hasMore && (
        <div className={s.loadMoreWrap}>
          <button className={s.loadMoreBtn} onClick={loadMore} disabled={loadingMore}>
            {loadingMore ? 'Loading...' : 'Load More'}
          </button>
        </div>
      )}
    </div>
  )
}
