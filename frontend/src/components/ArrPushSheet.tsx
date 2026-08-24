import { useState, useEffect } from 'preact/hooks'
import type { Title, TitleType, ArrTitleDetails } from '../types'
import { apiFetch } from '../api'
import { getName, getCoverUrl } from '../utils'
import { BottomSheet } from './BottomSheet'
import { TypeBadge } from './TypeBadge'
import { CoverPlaceholder, coverBackground } from './CoverPlaceholder'
import s from './ArrPushSheet.module.css'

interface RootFolder {
  id: number
  path: string
}

interface QualityProfile {
  id: number
  name: string
}

interface ArrSettings {
  radarr_std_monitored?: string
  radarr_std_search?: string
  radarr_std_root_folder?: string
  radarr_std_quality_profile?: string
  radarr_anime_monitored?: string
  radarr_anime_search?: string
  radarr_anime_root_folder?: string
  radarr_anime_quality_profile?: string
  sonarr_std_monitored?: string
  sonarr_std_search?: string
  sonarr_std_root_folder?: string
  sonarr_std_quality_profile?: string
  sonarr_anime_monitored?: string
  sonarr_anime_search?: string
  sonarr_anime_root_folder?: string
  sonarr_anime_quality_profile?: string
}

interface ArrPushSheetProps {
  open: boolean
  onClose: () => void
  title: Title | null
  onSuccess?: (arrId: number) => void
}

export function ArrPushSheet({ open, onClose, title, onSuccess }: ArrPushSheetProps) {
  if (!title) return null

  const isRadarr = title.type === 'movie'
  const app = isRadarr ? 'radarr' : 'sonarr'
  const appLabel = isRadarr ? 'Radarr' : 'Sonarr'
  const prefix = isRadarr
    ? (title.is_anime ? 'radarr_anime' : 'radarr_std')
    : (title.is_anime ? 'sonarr_anime' : 'sonarr_std')

  const hasRequiredID = isRadarr
    ? (title.tmdb_id != null && title.tmdb_id > 0)
    : (title.tvdb_id != null && title.tvdb_id > 0)

  const [loadingOptions, setLoadingOptions] = useState(false)
  const [rootFolders, setRootFolders] = useState<RootFolder[]>([])
  const [qualityProfiles, setQualityProfiles] = useState<QualityProfile[]>([])
  const [arrDetails, setArrDetails] = useState<ArrTitleDetails | null>(null)
  
  const [monitored, setMonitored] = useState('true')
  const [search, setSearch] = useState('false')
  const [rootFolder, setRootFolder] = useState('')
  const [qualityProfile, setQualityProfile] = useState('')

  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!open) {
      setError(null)
      return
    }

    setLoadingOptions(true)
    setError(null)

    Promise.all([
      apiFetch<ArrSettings>('/admin/arr').catch(() => ({} as ArrSettings)),
      apiFetch<RootFolder[]>(`/arr/${app}/rootfolder`).catch(() => [] as RootFolder[]),
      apiFetch<QualityProfile[]>(`/arr/${app}/qualityprofile`).catch(() => [] as QualityProfile[]),
      apiFetch<ArrTitleDetails>(`/arr/title/${title.id}`).catch(() => null),
    ]).then(([settings, rfList, qpList, liveDetails]) => {
      setRootFolders(rfList)
      setQualityProfiles(qpList)
      setArrDetails(liveDetails)

      if (liveDetails?.exists) {
        setMonitored(liveDetails.monitored ? 'true' : 'false')
        if (liveDetails.quality_profile_id > 0) {
          setQualityProfile(String(liveDetails.quality_profile_id))
        } else {
          setQualityProfile(qpList.length > 0 ? String(qpList[0].id) : '')
        }
        if (liveDetails.root_folder_path) {
          setRootFolder(liveDetails.root_folder_path)
        } else {
          setRootFolder(rfList.length > 0 ? rfList[0].path : '')
        }
      } else {
        const defMonitored = (settings as any)[`${prefix}_monitored`] || 'true'
        const defSearch = (settings as any)[`${prefix}_search`] || 'false'
        const defRoot = (settings as any)[`${prefix}_root_folder`] || (rfList.length > 0 ? rfList[0].path : '')
        const defQuality = (settings as any)[`${prefix}_quality_profile`] || (qpList.length > 0 ? String(qpList[0].id) : '')

        setMonitored(defMonitored)
        setSearch(defSearch)
        setRootFolder(defRoot)
        setQualityProfile(defQuality)
      }
      setLoadingOptions(false)
    }).catch(err => {
      setError(err.message)
      setLoadingOptions(false)
    })
  }, [open, app, prefix, title.id])

  const handleSave = async () => {
    if (!hasRequiredID) return
    if (!rootFolder || !qualityProfile) {
      setError('Please select a root folder and a quality profile.')
      return
    }

    setSaving(true)
    setError(null)

    try {
      const isExisting = arrDetails?.exists
      const endpoint = isExisting ? `/arr/title/${title.id}` : `/arr/push/${title.id}`
      const method = isExisting ? 'PUT' : 'POST'

      const res = await apiFetch<{ status?: string; arr_id?: number }>(endpoint, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          monitored: monitored === 'true',
          search: search === 'true',
          root_folder: rootFolder,
          quality_profile: parseInt(qualityProfile, 10),
        }),
      })
      const finalID = res.arr_id ?? arrDetails?.arr_id ?? 0
      onSuccess?.(finalID)
      onClose()
    } catch (err: any) {
      setError(err.message || `Error updating in ${appLabel}`)
    } finally {
      setSaving(false)
    }
  }

  const name = getName(title)
  const coverUrl = getCoverUrl(title.cover_url)
  const isLinked = arrDetails?.exists

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel={`Manage in ${appLabel}`}>
      <div className={s.sheet}>
        {/* Header with Poster & Essential info */}
        <div className={s.header}>
          <div
            className={s.posterWrap}
            style={{ background: coverBackground(coverUrl, title.type) }}
          >
            {coverUrl ? (
              <div
                className={s.poster}
                style={{ backgroundImage: `url(${coverUrl})` }}
              />
            ) : (
              <CoverPlaceholder type={title.type as TitleType} iconSize="24px" />
            )}
          </div>

          <div className={s.headerInfo}>
            <h2 className={s.title}>{name}</h2>
            <div className={s.badgesRow}>
              {title.year > 0 && <span className={s.year}>{title.year}</span>}
              <TypeBadge type={title.type as TitleType} size="sm" />
              <span className={`${s.serviceBadge} ${isRadarr ? s.radarrBadge : s.sonarrBadge}`}>
                {isLinked ? `Linked` : appLabel}
              </span>
            </div>
            {arrDetails?.web_url && (
              <a
                href={arrDetails.web_url}
                target="_blank"
                rel="noopener noreferrer"
                className={s.webLink}
              >
                <span>Open in {appLabel}</span>
                <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6" />
                  <polyline points="15 3 21 3 21 9" />
                  <line x1="10" y1="14" x2="21" y2="3" />
                </svg>
              </a>
            )}
          </div>
        </div>

        {/* Error / Warning notices */}
        {!hasRequiredID && (
          <div className={s.warningBox}>
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10" />
              <line x1="12" y1="8" x2="12" y2="12" />
              <line x1="12" y1="16" x2="12.01" y2="16" />
            </svg>
            <span>
              Missing {isRadarr ? 'TMDB' : 'TVDB'} ID. A rematch is required to link with {appLabel}.
            </span>
          </div>
        )}

        {error && (
          <div className={s.errorBanner}>
            {error}
          </div>
        )}

        {/* Configuration Options */}
        <div className={s.section}>
          <div className={s.sectionLabel}>{appLabel} Options</div>
          
          <div className={s.formGrid}>
            <div className={s.formGroup}>
              <label htmlFor="arr-root-folder" className={s.formLabel}>Root Folder</label>
              <select
                id="arr-root-folder"
                name="root_folder"
                className={s.select}
                value={rootFolder}
                disabled={loadingOptions || saving}
                onChange={e => setRootFolder((e.target as HTMLSelectElement).value)}
              >
                {rootFolders.length === 0 && <option value="">{rootFolder || 'No folders available'}</option>}
                {rootFolders.map(rf => (
                  <option key={rf.id} value={rf.path}>{rf.path}</option>
                ))}
              </select>
            </div>

            <div className={s.formGroup}>
              <label htmlFor="arr-quality-profile" className={s.formLabel}>Quality Profile</label>
              <select
                id="arr-quality-profile"
                name="quality_profile"
                className={s.select}
                value={qualityProfile}
                disabled={loadingOptions || saving}
                onChange={e => setQualityProfile((e.target as HTMLSelectElement).value)}
              >
                {qualityProfiles.length === 0 && <option value="">No profiles available</option>}
                {qualityProfiles.map(qp => (
                  <option key={qp.id} value={qp.id}>{qp.name}</option>
                ))}
              </select>
            </div>

            <div className={s.formGroup}>
              <label htmlFor="arr-monitored" className={s.formLabel}>Monitored</label>
              <select
                id="arr-monitored"
                name="monitored"
                className={s.select}
                value={monitored}
                disabled={loadingOptions || saving}
                onChange={e => setMonitored((e.target as HTMLSelectElement).value)}
              >
                <option value="true">Yes</option>
                <option value="false">No</option>
              </select>
            </div>

            <div className={s.formGroup}>
              <label htmlFor="arr-search" className={s.formLabel}>Search for Missing</label>
              <select
                id="arr-search"
                name="search"
                className={s.select}
                value={search}
                disabled={loadingOptions || saving}
                onChange={e => setSearch((e.target as HTMLSelectElement).value)}
              >
                <option value="false">No</option>
                <option value="true">Yes</option>
              </select>
            </div>
          </div>
        </div>

        {/* Primary Action Button */}
        <div className={s.actionWrap}>
          <button
            type="button"
            className={`${s.btnPush} ${isRadarr ? s.btnRadarr : s.btnSonarr}`}
            onClick={handleSave}
            disabled={!hasRequiredID || saving || loadingOptions || !rootFolder || !qualityProfile}
          >
            {saving ? (
              'Saving...'
            ) : isLinked ? (
              <>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" />
                  <polyline points="17 21 17 13 7 13 7 21" />
                  <polyline points="7 3 7 8 15 8" />
                </svg>
                Update in {appLabel}
              </>
            ) : (
              <>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="22" y1="2" x2="11" y2="13" />
                  <polygon points="22 2 15 22 11 13 2 9 22 2" />
                </svg>
                Send to {appLabel}
              </>
            )}
          </button>
        </div>
      </div>
    </BottomSheet>
  )
}
