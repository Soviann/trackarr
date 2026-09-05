import { render, screen, waitFor, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { Admin } from './Admin'
import { apiFetch } from '../api'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('Admin Dashboard Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it('renders all three categories and cards with badges', async () => {
    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path === '/admin/counts') {
        return {
          pending_validations: 4,
          dead_tasks: 2,
        }
      }
      if (path === '/admin/auth-settings') {
        return {
          auth_mode: 'hybrid',
          has_password: true,
          has_google: true,
        }
      }
      if (path === '/admin/system-settings') {
        return {
          tmdb_configured: true,
          tvdb_configured: true,
          radarr_configured: true,
          sonarr_configured: true,
          anilist_configured: true,
          vapid_configured: true,
        }
      }
      return {}
    })

    render(<Admin />)

    await waitFor(() => {
      expect(screen.getByText('Admin Dashboard')).not.toBeNull()
    })

    // Section 1
    expect(screen.getByText('Activity & Immediate Actions')).not.toBeNull()
    expect(screen.getByText('Match Validations')).not.toBeNull()
    expect(screen.getByText('4')).not.toBeNull()
    expect(screen.getByText('Background Tasks & Errors')).not.toBeNull()
    expect(screen.getByText('2')).not.toBeNull()
    expect(screen.getByText('Season & Anime Audit')).not.toBeNull()

    // Section 2
    expect(screen.getByText('Integrations & External Services')).not.toBeNull()
    expect(screen.getByText('System Settings & API Keys')).not.toBeNull()
    expect(screen.getByText('Arr Stack')).not.toBeNull()
    expect(screen.getByText('Radarr / Sonarr / Prowlarr')).not.toBeNull()
    expect(screen.getByText('AniList Synchronization')).not.toBeNull()
    expect(screen.getByText('Web Push Notifications')).not.toBeNull()

    // Section 3
    expect(screen.getByText('Security & Maintenance')).not.toBeNull()
    expect(screen.getByText('Authentication & Access')).not.toBeNull()
    expect(screen.getByText('Hybrid Mode')).not.toBeNull()
    expect(screen.getByText('Documentation & Help')).not.toBeNull()
    expect(screen.getByText('Refresh All Metadata')).not.toBeNull()

    // Section 4: Data Management & Backups
    expect(screen.getByText('Backup & Data Management')).not.toBeNull()
    expect(screen.getByText('1-Click Library Export')).not.toBeNull()
    expect(screen.getByText('Full JSON')).not.toBeNull()
    expect(screen.getByText('Spreadsheet CSV')).not.toBeNull()
    expect(screen.getByText('Trakt.tv Sync')).not.toBeNull()
    expect(screen.getByText('Import Backup Archive')).not.toBeNull()
    expect(screen.getByText('Drag & drop backup file or click to browse')).not.toBeNull()
  })
})
