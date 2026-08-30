import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { AdminSettings } from './AdminSettings'
import { apiFetch } from '../api'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('AdminSettings Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    setLocale('en')
  })

  afterEach(() => {
    cleanup()
  })

  it('loads and displays system settings', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tmdb_api_key: '••••••••abcd',
      tmdb_configured: true,
      tvdb_api_key: '',
      tvdb_configured: false,
      gemini_api_keys: '',
      gemini_configured: false,
      anilist_client_id: '',
      anilist_client_secret: '',
      anilist_configured: false,
      jellyfin_webhook_secret: 'jellyfin123',
      jellyfin_webhook_url: 'http://localhost:8080/api/webhook/jellyfin/jellyfin123',
      plex_webhook_secret: 'plex456',
      plex_webhook_url: 'http://localhost:8080/api/webhook/plex/plex456',
      radarr_url: 'http://192.168.1.50:7878',
      radarr_api_key: '••••••••',
      radarr_configured: true,
      sonarr_url: '',
      sonarr_api_key: '',
      sonarr_configured: false,
      prowlarr_url: '',
      prowlarr_api_key: '',
      prowlarr_configured: false,
      vapid_public_key: '',
      vapid_subject: '',
      vapid_configured: false,
    })

    render(<AdminSettings />)

    await waitFor(() => {
      expect(screen.getByText('System Settings & API Keys')).not.toBeNull()
    })

    expect(screen.getByText('🎬 Metadata & Artificial Intelligence')).not.toBeNull()
    expect(screen.getByText('📺 Media Servers & Webhooks')).not.toBeNull()
    expect(screen.getByText('📦 Download Stack (Radarr / Sonarr / Prowlarr)')).not.toBeNull()
    expect(screen.getByText('🔔 Web Push Notifications (VAPID)')).not.toBeNull()
  })

  it('handles testing TMDB connection', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce({
        tmdb_api_key: '••••••••abcd',
        tmdb_configured: true,
      })
      .mockResolvedValueOnce({
        ok: true,
        message: 'TMDB connection successful (valid key)',
      })

    render(<AdminSettings />)

    await waitFor(() => {
      expect(screen.getByText('System Settings & API Keys')).not.toBeNull()
    })

    const testBtns = screen.getAllByText('Test')
    fireEvent.click(testBtns[0])

    await waitFor(() => {
      expect(screen.getByText(/TMDB connection successful/)).not.toBeNull()
    })
  })

  it('allows switching interface language between English and French', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tmdb_api_key: '',
      tmdb_configured: false,
    })

    render(<AdminSettings />)

    await waitFor(() => {
      expect(screen.getAllByText('Français').length).toBeGreaterThan(0)
    })

    const francaisBtns = screen.getAllByText('Français')
    fireEvent.click(francaisBtns[0])

    await waitFor(() => {
      expect(screen.getByText(/Apparence & Thèmes/)).not.toBeNull()
    })
  })

  it('allows selecting primary metadata language and saving settings', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce({
        tmdb_api_key: '',
        tmdb_configured: false,
        metadata_language: 'fr',
      })
      .mockResolvedValueOnce({ ok: true })
      .mockResolvedValueOnce({
        tmdb_api_key: '',
        tmdb_configured: false,
        metadata_language: 'en',
      })

    render(<AdminSettings />)

    await waitFor(() => {
      expect(screen.getByText(/Primary Metadata Language/)).not.toBeNull()
    })

    // Find the English metadata button (within Metadata section, after Interface English)
    const englishBtns = screen.getAllByText('English')
    expect(englishBtns.length).toBeGreaterThan(0)
    fireEvent.click(englishBtns[englishBtns.length - 1])

    const saveBtn = screen.getByText('Save Settings')
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/system-settings', expect.objectContaining({
        method: 'PUT',
        body: expect.stringContaining('"metadata_language":"en"'),
      }))
    })
  })

  it('allows toggling streaming watch providers and saving settings', async () => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce({
        tmdb_api_key: '',
        tmdb_configured: false,
        enabled_watch_providers: 'netflix,prime,disney,apple,max,canal,crunchyroll,paramount,adn',
      })
      .mockResolvedValueOnce({ ok: true })
      .mockResolvedValueOnce({
        tmdb_api_key: '',
        tmdb_configured: false,
        enabled_watch_providers: 'netflix,prime',
      })

    render(<AdminSettings />)

    await waitFor(() => {
      expect(screen.getByText(/Streaming Platforms & Watch Providers/)).not.toBeNull()
    })

    // Find and toggle Netflix button
    const netflixBtn = screen.getByText('Netflix').closest('button')
    expect(netflixBtn).not.toBeNull()
    fireEvent.click(netflixBtn!)

    const saveBtn = screen.getByText('Save Settings')
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/system-settings', expect.objectContaining({
        method: 'PUT',
        body: expect.stringContaining('enabled_watch_providers'),
      }))
    })
  })
})
