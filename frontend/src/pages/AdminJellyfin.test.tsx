import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { AdminJellyfin } from './AdminJellyfin'
import { apiFetch } from '../api'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('AdminJellyfin (Media Servers) Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    setLocale('en')
    Object.assign(navigator, {
      clipboard: {
        writeText: vi.fn().mockResolvedValue(undefined),
      },
    })
  })

  afterEach(() => {
    cleanup()
  })

  const mockSettings = {
    jellyfin_configured: true,
    jellyfin_last_scrobble_at: '2026-10-01T20:15:00Z',
    plex_last_scrobble_at: '2026-10-02T22:30:00Z',
  }

  const mockSysSettings = {
    jellyfin_webhook_secret: 'jf-secret-xyz',
    jellyfin_webhook_url: 'https://trackarr.local/api/webhook/jellyfin/jf-secret-xyz',
    plex_webhook_secret: 'plex-secret-abc',
    plex_webhook_url: 'https://trackarr.local/api/webhook/plex/plex-secret-abc',
  }

  it('renders Jellyfin and Plex sections with status, secrets, and instructions', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/settings') return mockSettings
      if (url === '/admin/system-settings') return mockSysSettings
      return {}
    })

    render(<AdminJellyfin />)

    await waitFor(() => {
      expect(screen.getByText('Media Servers & Webhooks')).toBeTruthy()
    })

    expect(screen.getByRole('heading', { name: /Jellyfin/, level: 2 })).toBeTruthy()
    expect(screen.getByRole('heading', { name: /Plex/, level: 2 })).toBeTruthy()
    expect(screen.getByDisplayValue('jf-secret-xyz')).toBeTruthy()
    expect(screen.getByDisplayValue('plex-secret-abc')).toBeTruthy()
    expect(screen.getByDisplayValue('https://trackarr.local/api/webhook/jellyfin/jf-secret-xyz')).toBeTruthy()
    expect(screen.getByDisplayValue('https://trackarr.local/api/webhook/plex/plex-secret-abc')).toBeTruthy()
    expect(screen.getByText('Webhook Configuration in Jellyfin')).toBeTruthy()
    expect(
      screen.getByText(/Notification Types: Playback Start, Playback Stop, User Data Saved, Item Added/)
    ).toBeTruthy()
    expect(screen.getByText('Webhook Configuration in Plex')).toBeTruthy()
  })

  it('copies webhook URL to clipboard upon clicking Copy URL', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/settings') return mockSettings
      if (url === '/admin/system-settings') return mockSysSettings
      return {}
    })

    render(<AdminJellyfin />)

    await waitFor(() => {
      expect(screen.getAllByRole('button', { name: /Copy URL/i }).length).toBe(2)
    })

    const copyButtons = screen.getAllByRole('button', { name: /Copy URL/i })
    fireEvent.click(copyButtons[0])

    expect(navigator.clipboard.writeText).toHaveBeenCalledWith(
      'https://trackarr.local/api/webhook/jellyfin/jf-secret-xyz'
    )
  })

  it('saves updated secrets on Save Settings click', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/settings') return mockSettings
      if (url === '/admin/system-settings') return mockSysSettings
      return {}
    })

    render(<AdminJellyfin />)

    await waitFor(() => {
      expect(screen.getByDisplayValue('jf-secret-xyz')).toBeTruthy()
    })

    const jfInput = screen.getByDisplayValue('jf-secret-xyz')
    fireEvent.input(jfInput, { target: { value: 'new-jf-secret' } })

    const saveBtn = screen.getByRole('button', { name: /Save Settings/i })
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/system-settings', {
        method: 'PUT',
        body: JSON.stringify({
          jellyfin_webhook_secret: 'new-jf-secret',
          jellyfin_url: '',
          jellyfin_api_key: '',
          jellyfin_user_id: '',
          plex_webhook_secret: 'plex-secret-abc',
        }),
      })
    })

    await waitFor(() => {
      expect(screen.getByText('Media server settings saved successfully!')).toBeTruthy()
    })
  })

  it('displays None recorded yet when last scrobble timestamps are null', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/settings') {
        return {
          jellyfin_configured: false,
          jellyfin_last_scrobble_at: null,
          plex_last_scrobble_at: null,
        }
      }
      if (url === '/admin/system-settings') return mockSysSettings
      return {}
    })

    render(<AdminJellyfin />)

    await waitFor(() => {
      expect(screen.getAllByText('None recorded').length).toBe(2)
    })
  })

  it('displays error banner when saving media server settings fails', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string, opts?: { method?: string }) => {
      if (url === '/settings') return mockSettings
      if (url === '/admin/system-settings' && !opts?.method) return mockSysSettings
      if (url === '/admin/system-settings' && opts?.method === 'PUT') {
        throw new Error('Save failed: unauthorized')
      }
      return {}
    })

    render(<AdminJellyfin />)

    await waitFor(() => {
      expect(screen.getByDisplayValue('jf-secret-xyz')).toBeTruthy()
    })

    const saveBtn = screen.getByRole('button', { name: /Save Settings/i })
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(screen.getByText('Save failed: unauthorized')).toBeTruthy()
    })
  })

  it('disables Sync Unwatched Movies button when Jellyfin API is not configured', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/settings') return mockSettings
      if (url === '/admin/system-settings') {
        return { ...mockSysSettings, jellyfin_api_configured: false }
      }
      return {}
    })

    render(<AdminJellyfin />)

    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Sync Unwatched Movies' })).toBeTruthy()
    })

    const syncBtn = screen.getByRole('button', { name: 'Sync Unwatched Movies' }) as HTMLButtonElement
    expect(syncBtn.disabled).toBe(true)
  })

  it('syncs unwatched movies, shows loading state, and renders localized summary', async () => {
    let resolveSync: ((value: unknown) => void) | undefined
    const pendingSync = new Promise((resolve) => {
      resolveSync = resolve
    })

    vi.mocked(apiFetch).mockImplementation((url: string, opts?: { method?: string }) => {
      if (url === '/settings') return Promise.resolve(mockSettings)
      if (url === '/admin/system-settings') {
        return Promise.resolve({ ...mockSysSettings, jellyfin_api_configured: true })
      }
      if (url === '/admin/jellyfin/sync-movies' && opts?.method === 'POST') {
        return pendingSync
      }
      return Promise.resolve({})
    })

    render(<AdminJellyfin />)

    await waitFor(() => {
      const btn = screen.getByRole('button', { name: 'Sync Unwatched Movies' }) as HTMLButtonElement
      expect(btn.disabled).toBe(false)
    })

    const syncBtn = screen.getByRole('button', { name: 'Sync Unwatched Movies' })
    fireEvent.click(syncBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/jellyfin/sync-movies', {
        method: 'POST',
        body: JSON.stringify({ dry_run: false }),
      })
      const loadingBtn = screen.getByRole('button', { name: 'Syncing movies...' }) as HTMLButtonElement
      expect(loadingBtn.disabled).toBe(true)
    })

    resolveSync?.({
      dry_run: false,
      fetched: 12,
      unwatched: 5,
      created: 3,
      skipped: 2,
      errors: 0,
    })

    await waitFor(() => {
      const statusAlert = screen.getByRole('status')
      expect(statusAlert.textContent).toBe(
        'Fetched: 12 · Unwatched: 5 · Created: 3 · Skipped: 2 · Errors: 0'
      )
      const doneBtn = screen.getByRole('button', { name: 'Sync Unwatched Movies' }) as HTMLButtonElement
      expect(doneBtn.disabled).toBe(false)
    })
  })

  it('displays error banner when syncing unwatched movies fails', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string, opts?: { method?: string }) => {
      if (url === '/settings') return mockSettings
      if (url === '/admin/system-settings') {
        return { ...mockSysSettings, jellyfin_api_configured: true }
      }
      if (url === '/admin/jellyfin/sync-movies' && opts?.method === 'POST') {
        throw new Error('Failed to sync movies from Jellyfin')
      }
      return {}
    })

    render(<AdminJellyfin />)

    await waitFor(() => {
      const btn = screen.getByRole('button', { name: 'Sync Unwatched Movies' }) as HTMLButtonElement
      expect(btn.disabled).toBe(false)
    })

    const syncBtn = screen.getByRole('button', { name: 'Sync Unwatched Movies' })
    fireEvent.click(syncBtn)

    await waitFor(() => {
      const alert = screen.getByRole('alert')
      expect(alert.textContent).toBe('Failed to sync movies from Jellyfin')
      const btn = screen.getByRole('button', { name: 'Sync Unwatched Movies' }) as HTMLButtonElement
      expect(btn.disabled).toBe(false)
    })

    expect(screen.queryByRole('status')).toBeNull()
  })
})
