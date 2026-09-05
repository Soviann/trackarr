import { useState, useRef } from 'preact/hooks'
import { route } from 'preact-router'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { colors } from '../theme'
import { routeTo } from '../routes'
import { useTranslation } from '../i18n'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import { BottomSheet } from '../components/BottomSheet'
import s from './Admin.module.css'

interface AdminCounts {
  pending_validations: number
  dead_tasks: number
}

interface AuthSettings {
  auth_mode: 'google' | 'password' | 'hybrid'
  has_password: boolean
  has_google: boolean
}

interface SystemSettings {
  tmdb_configured: boolean
  tvdb_configured: boolean
  gemini_configured: boolean
  anilist_configured: boolean
  radarr_configured: boolean
  sonarr_configured: boolean
  prowlarr_configured: boolean
  vapid_configured: boolean
}

interface BackupSummaryResult {
  dry_run: boolean
  created: number
  skipped: number
  errors: number
  total: number
  message?: string
}

export function Admin({ path }: { path?: string }) {
  const { t } = useTranslation()
  const { data: counts } = useApi<AdminCounts>('/admin/counts')
  const { data: authSettings } = useApi<AuthSettings>('/admin/auth-settings')
  const { data: sysSettings } = useApi<SystemSettings>('/admin/system-settings')

  const [refreshing, setRefreshing] = useState(false)
  const [showRefreshModal, setShowRefreshModal] = useState(false)
  const [refreshSuccess, setRefreshSuccess] = useState(false)

  // Backup & Import states
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [isDragging, setIsDragging] = useState(false)
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [previewResult, setPreviewResult] = useState<BackupSummaryResult | null>(null)
  const [showPreviewModal, setShowPreviewModal] = useState(false)
  const [importing, setImporting] = useState(false)
  const [importSuccessMsg, setImportSuccessMsg] = useState<string | null>(null)
  const [importErrorMsg, setImportErrorMsg] = useState<string | null>(null)

  const handleRefreshAll = async () => {
    setRefreshing(true)
    try {
      await apiFetch('/admin/refresh-all', { method: 'POST' })
      setRefreshSuccess(true)
      setTimeout(() => setRefreshSuccess(false), 4000)
    } finally {
      setRefreshing(false)
    }
  }

  const runPreview = async (file: File) => {
    setSelectedFile(file)
    setImporting(true)
    setImportErrorMsg(null)
    setImportSuccessMsg(null)
    try {
      const formData = new FormData()
      formData.append('file', file)
      const res = await apiFetch<BackupSummaryResult>('/admin/import?dry_run=true', {
        method: 'POST',
        body: formData,
      })
      setPreviewResult(res)
      setShowPreviewModal(true)
    } catch (err: unknown) {
      setImportErrorMsg(err instanceof Error ? err.message : String(err))
    } finally {
      setImporting(false)
    }
  }

  const handleExecuteImport = async () => {
    if (!selectedFile) return
    setImporting(true)
    try {
      const formData = new FormData()
      formData.append('file', selectedFile)
      const res = await apiFetch<BackupSummaryResult>('/admin/import?dry_run=false', {
        method: 'POST',
        body: formData,
      })
      setShowPreviewModal(false)
      setSelectedFile(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
      setImportSuccessMsg(
        t('admin.importSuccessMsg', {
          created: String(res.created),
          skipped: String(res.skipped),
        })
      )
      setTimeout(() => setImportSuccessMsg(null), 6000)
    } catch (err: unknown) {
      setImportErrorMsg(err instanceof Error ? err.message : String(err))
    } finally {
      setImporting(false)
    }
  }

  const handleDragOver = (e: DragEvent) => {
    e.preventDefault()
    setIsDragging(true)
  }

  const handleDragLeave = () => {
    setIsDragging(false)
  }

  const handleDrop = (e: DragEvent) => {
    e.preventDefault()
    setIsDragging(false)
    if (e.dataTransfer?.files && e.dataTransfer.files.length > 0) {
      const file = e.dataTransfer.files[0]
      runPreview(file)
    }
  }

  const handleFileSelect = (e: Event) => {
    const input = e.target as HTMLInputElement
    if (input.files && input.files.length > 0) {
      const file = input.files[0]
      runPreview(file)
    }
  }

  // Auth mode badge label
  const authModeLabel = authSettings
    ? authSettings.auth_mode === 'hybrid'
      ? 'Hybrid Mode'
      : authSettings.auth_mode === 'google'
        ? 'Google OAuth'
        : 'Credentials'
    : 'Secured'

  // Arr stack status badge
  const arrStatusLabel = sysSettings
    ? sysSettings.radarr_configured || sysSettings.sonarr_configured
      ? 'Connected'
      : 'Not configured'
    : 'Configuration'

  const arrStatusClass = sysSettings && (sysSettings.radarr_configured || sysSettings.sonarr_configured)
    ? s.active
    : ''

  return (
    <div className={s.page}>
      {/* HEADER */}
      <div className={s.header}>
        <div className={s.headerLeft}>
          <h1 className={s.title}>Admin Dashboard</h1>
          <div className={s.subtitle}>Trackarr • Personal instance</div>
        </div>
        <div className={s.systemStatusPill}>
          <div className={s.statusDot} />
          <span>Online</span>
        </div>
      </div>

      {/* SECTION 1: ACTIVITY & IMMEDIATE ACTIONS */}
      <div className={s.section}>
        <div className={s.sectionHeader}>
          <span className={s.sectionIcon}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2" />
            </svg>
          </span>
          <span className={s.sectionTitle}>Activity & Immediate Actions</span>
        </div>

        <div className={s.cardGroup}>
          {/* Validations */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminValidate())}
            style={{ '--card-color': colors.accent, '--badge-bg': colors.accent, '--badge-color': '#000' } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
                <polyline points="22 4 12 14.01 9 11.01" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>Match Validations</span>
                {counts && counts.pending_validations > 0 && (
                  <span className={s.badgeCount}>{counts.pending_validations}</span>
                )}
              </div>
              <span className={s.cardDesc}>Titles pending confirmation or manual reconciliation</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>

          {/* Tasks Queue & Errors */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminTasks())}
            style={{ '--card-color': colors.statusCrit, '--badge-bg': colors.statusCrit, '--badge-color': '#fff' } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>Background Tasks & Errors</span>
                {counts && counts.dead_tasks > 0 && (
                  <span className={s.badgeCount}>{counts.dead_tasks}</span>
                )}
              </div>
              <span className={s.cardDesc}>Asynchronous enrichment queue and diagnostics</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>

          {/* Season Audit */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminSeasonAudit())}
            style={{ '--card-color': colors.brandAnilist } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="2" y="3" width="20" height="5" rx="2" />
                <rect x="2" y="10" width="20" height="5" rx="2" />
                <rect x="2" y="17" width="20" height="5" rx="2" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>Season & Anime Audit</span>
                <span className={`${s.badgeStatus} ${s.active}`}>Up to date</span>
              </div>
              <span className={s.cardDesc}>Detect and merge split anime seasons (AniList Parts)</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>
        </div>
      </div>

      {/* SECTION 2: INTEGRATIONS & CONFIGURATION */}
      <div className={s.section}>
        <div className={s.sectionHeader}>
          <span className={s.sectionIcon}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <circle cx="12" cy="12" r="3" />
              <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
            </svg>
          </span>
          <span className={s.sectionTitle}>Integrations & External Services</span>
        </div>

        <div className={s.cardGroup}>
          {/* System Settings & API Keys */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminSettings())}
            style={{ '--card-color': colors.accent } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 2v4M12 18v4M4.93 4.93l2.83 2.83M16.24 16.24l2.83 2.83M2 12h4M18 12h4M4.93 19.07l2.83-2.83M16.24 7.76l2.83-2.83" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>System Settings & API Keys</span>
                <span className={`${s.badgeStatus} ${s.active}`}>TMDB • TVDB • Webhooks</span>
              </div>
              <span className={s.cardDesc}>Manage API keys, Gemini AI, secrets and webhook URLs</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>

          {/* Radarr / Sonarr / Prowlarr defaults */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminArr())}
            style={{ '--card-color': colors.statusWarn } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="2" x2="12" y2="22" />
                <line x1="2" y1="12" x2="22" y2="12" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>Arr Stack</span>
                <span className={`${s.badgeStatus} ${arrStatusClass}`}>{arrStatusLabel}</span>
              </div>
              <span className={s.cardDesc}>Radarr / Sonarr / Prowlarr</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>

          {/* AniList Sync */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminAniList())}
            style={{ '--card-color': colors.brandAnilist } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
                <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>AniList Synchronization</span>
                <span className={`${s.badgeStatus} ${sysSettings?.anilist_configured ? s.active : ''}`}>
                  {sysSettings?.anilist_configured ? 'Synced' : 'Optional'}
                </span>
              </div>
              <span className={s.cardDesc}>OAuth account connection, watched episodes and rating push</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>

          {/* Notifications */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminNotifications())}
            style={{ '--card-color': colors.accent } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
                <path d="M13.73 21a2 2 0 0 1-3.46 0" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>Web Push Notifications</span>
                <span className={`${s.badgeStatus} ${sysSettings?.vapid_configured ? s.active : ''}`}>
                  {sysSettings?.vapid_configured ? 'Enabled' : 'Optional'}
                </span>
              </div>
              <span className={s.cardDesc}>Scrobble alerts, rating prompts and library recaps</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>
        </div>
      </div>

      {/* SECTION 3: SECURITY & MAINTENANCE */}
      <div className={s.section}>
        <div className={s.sectionHeader}>
          <span className={s.sectionIcon}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
              <path d="M7 11V7a5 5 0 0 1 10 0v4" />
            </svg>
          </span>
          <span className={s.sectionTitle}>Security & Maintenance</span>
        </div>

        <div className={s.cardGroup}>
          {/* Auth & Security */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminAuth())}
            style={{ '--card-color': colors.accent } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>Authentication & Access</span>
                <span className={`${s.badgeStatus} ${s.active}`}>{authModeLabel}</span>
              </div>
              <span className={s.cardDesc}>Login mode, admin password and emergency recovery key</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>

          {/* Help */}
          <button
            type="button"
            className={s.card}
            onClick={() => route(routeTo.adminHelp())}
            style={{ '--card-color': colors.inkDim } as Record<string, string>}
          >
            <div className={s.cardIconWrap}>
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10" />
                <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
                <line x1="12" y1="17" x2="12.01" y2="17" />
              </svg>
            </div>
            <div className={s.cardContent}>
              <div className={s.cardTop}>
                <span className={s.cardLabel}>Documentation & Help</span>
              </div>
              <span className={s.cardDesc}>Matching engine details, shortcuts and usage guide</span>
            </div>
            <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="9 18 15 12 9 6" />
            </svg>
          </button>
        </div>

        {/* REFRESH ACTION BOX */}
        <div className={s.refreshBox}>
          <div className={s.refreshInfo}>
            <div className={s.refreshTitle}>{t('admin.refreshAll')}</div>
            <div className={s.refreshDesc}>
              {refreshSuccess
                ? '✅ Background metadata refresh started successfully!'
                : 'Updates synopsis, ratings, posters, and cast for all titles in the background.'}
            </div>
          </div>
          <button
            type="button"
            className={s.refreshBtn}
            onClick={() => setShowRefreshModal(true)}
            disabled={refreshing}
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <polyline points="23 4 23 10 17 10" />
              <polyline points="1 20 1 14 7 14" />
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
            </svg>
            <span>{refreshing ? 'Refreshing...' : 'Refresh'}</span>
          </button>
        </div>
      </div>

      {/* SECTION 4: DATA MANAGEMENT & BACKUPS */}
      <div className={s.section}>
        <div className={s.sectionHeader}>
          <span className={s.sectionIcon}>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
              <polyline points="7 10 12 15 17 10" />
              <line x1="12" y1="15" x2="12" y2="3" />
            </svg>
          </span>
          <span className={s.sectionTitle}>{t('admin.dataManagement')}</span>
        </div>

        {importSuccessMsg && (
          <div className={`${s.alertBanner} ${s.alertSuccess}`}>
            <span>✅ {importSuccessMsg}</span>
          </div>
        )}

        {importErrorMsg && (
          <div className={`${s.alertBanner} ${s.alertError}`}>
            <span>⚠️ {importErrorMsg}</span>
          </div>
        )}

        <div className={s.backupBox}>
          {/* 1-Click Export */}
          <div className={s.backupHeader}>
            <span className={s.backupSubTitle}>{t('admin.exportTitle')}</span>
            <span className={s.backupSubDesc}>{t('admin.exportDesc')}</span>
          </div>

          <div className={s.exportButtonsRow}>
            <a href="/api/admin/export/json" download className={s.exportBtn}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                <polyline points="14 2 14 8 20 8" />
                <line x1="12" y1="18" x2="12" y2="12" />
                <line x1="9" y1="15" x2="15" y2="15" />
              </svg>
              <span>{t('admin.exportJson')}</span>
            </a>

            <a href="/api/admin/export/csv" download className={s.exportBtn}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
                <polyline points="14 2 14 8 20 8" />
                <rect x="8" y="12" width="8" height="6" />
              </svg>
              <span>{t('admin.exportCsv')}</span>
            </a>

            <a href="/api/admin/export/trakt" download className={s.exportBtn}>
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="23 4 23 10 17 10" />
                <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
              </svg>
              <span>{t('admin.exportTrakt')}</span>
            </a>
          </div>

          <div className={s.importDivider} />

          {/* Import Dropzone */}
          <div className={s.backupHeader}>
            <span className={s.backupSubTitle}>{t('admin.importTitle')}</span>
            <span className={s.backupSubDesc}>{t('admin.importDesc')}</span>
          </div>

          <div
            className={`${s.dropzone} ${isDragging ? s.dropzoneActive : ''}`}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
            onClick={() => fileInputRef.current?.click()}
          >
            <input
              ref={fileInputRef}
              type="file"
              accept=".zip,.json,.csv"
              style={{ display: 'none' }}
              onChange={handleFileSelect}
            />
            <div className={s.dropzoneIcon}>
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                <polyline points="17 8 12 3 7 8" />
                <line x1="12" y1="3" x2="12" y2="15" />
              </svg>
            </div>
            <div className={s.dropzoneText}>
              <span className={s.dropzonePrimary}>{t('admin.importDropzone')}</span>
              <span className={s.dropzoneSecondary}>{t('admin.importFormats')}</span>
            </div>
          </div>
        </div>
      </div>

      {/* REFRESH CONFIRMATION */}
      <ConfirmationDrawer
        open={showRefreshModal}
        onClose={() => setShowRefreshModal(false)}
        onConfirm={handleRefreshAll}
        title="Refresh all metadata?"
        description="This operation runs in the background and may take several minutes depending on your library size."
        confirmText="Refresh"
        cancelText="Cancel"
      />

      {/* DRY-RUN PREVIEW BOTTOM SHEET */}
      <BottomSheet
        open={showPreviewModal}
        onClose={() => {
          if (!importing) {
            setShowPreviewModal(false)
            setSelectedFile(null)
          }
        }}
        ariaLabel={t('admin.importPreviewTitle')}
      >
        <div className={s.previewSheet}>
          <div className={s.previewTitle}>{t('admin.importPreviewTitle')}</div>
          <div className={s.previewDesc}>{t('admin.importPreviewDesc')}</div>

          {selectedFile && (
            <div className={s.previewFileBadge}>
              📁 <span>{selectedFile.name}</span> ({(selectedFile.size / 1024).toFixed(1)} KB)
            </div>
          )}

          {previewResult && (
            <div className={s.previewGrid}>
              <div className={s.previewStatCard}>
                <span className={s.previewStatNum}>{previewResult.total}</span>
                <span className={s.previewStatLabel}>{t('admin.importTotal')}</span>
              </div>
              <div className={s.previewStatCard}>
                <span className={`${s.previewStatNum} ${s.previewStatNumCreated}`}>
                  +{previewResult.created}
                </span>
                <span className={s.previewStatLabel}>{t('admin.importToCreate')}</span>
              </div>
              <div className={s.previewStatCard}>
                <span className={`${s.previewStatNum} ${s.previewStatNumSkipped}`}>
                  {previewResult.skipped}
                </span>
                <span className={s.previewStatLabel}>{t('admin.importToSkip')}</span>
              </div>
            </div>
          )}

          <div className={s.previewActions}>
            <button
              type="button"
              className={s.previewCancelBtn}
              onClick={() => {
                setShowPreviewModal(false)
                setSelectedFile(null)
              }}
              disabled={importing}
            >
              Cancel
            </button>
            <button
              type="button"
              className={s.previewConfirmBtn}
              onClick={handleExecuteImport}
              disabled={importing || previewResult?.created === 0}
            >
              {importing ? (
                <>
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" style={{ animation: 'spin 1s linear infinite' }}>
                    <line x1="12" y1="2" x2="12" y2="6" />
                    <line x1="12" y1="18" x2="12" y2="22" />
                    <line x1="4.93" y1="4.93" x2="7.76" y2="7.76" />
                    <line x1="16.24" y1="16.24" x2="19.07" y2="19.07" />
                    <line x1="2" y1="12" x2="6" y2="12" />
                    <line x1="18" y1="12" x2="22" y2="12" />
                  </svg>
                  <span>{t('admin.importRunning')}</span>
                </>
              ) : (
                <span>{t('admin.importConfirmBtn')}</span>
              )}
            </button>
          </div>
        </div>
      </BottomSheet>
    </div>
  )
}
