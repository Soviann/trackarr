import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { colors } from '../theme'
import { routeTo } from '../routes'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
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

export function Admin({ path }: { path?: string }) {
  const { data: counts } = useApi<AdminCounts>('/admin/counts')
  const { data: authSettings } = useApi<AuthSettings>('/admin/auth-settings')
  const { data: sysSettings } = useApi<SystemSettings>('/admin/system-settings')

  const [refreshing, setRefreshing] = useState(false)
  const [showRefreshModal, setShowRefreshModal] = useState(false)
  const [refreshSuccess, setRefreshSuccess] = useState(false)

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
            <div className={s.refreshTitle}>Refresh All Metadata</div>
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

      <ConfirmationDrawer
        open={showRefreshModal}
        onClose={() => setShowRefreshModal(false)}
        onConfirm={handleRefreshAll}
        title="Refresh all metadata?"
        description="This operation runs in the background and may take several minutes depending on your library size."
        confirmText="Refresh"
        cancelText="Cancel"
      />
    </div>
  )
}
