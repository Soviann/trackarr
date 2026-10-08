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
})
