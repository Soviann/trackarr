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
    expect(screen.getByText('🎨 Appearance & Themes')).not.toBeNull()
    expect(screen.getByText('📺 Streaming Platforms & Watch Providers')).not.toBeNull()
    expect(screen.queryByText('📺 Media Servers & Webhooks')).toBeNull()
    expect(screen.queryByText('📦 Download Stack (Radarr / Sonarr / Prowlarr)')).toBeNull()
    expect(screen.queryByText('🔔 Web Push Notifications (VAPID)')).toBeNull()
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
      expect(screen.getByTestId('interface-language-dropdown')).not.toBeNull()
    })

    fireEvent.click(screen.getByTestId('interface-language-dropdown'))

    const francaisOption = screen.getByRole('option', { name: /Français/ }) // i18n-ignore
    fireEvent.click(francaisOption)

    await waitFor(() => {
      expect(screen.getByText(/Apparence & Thèmes/)).not.toBeNull() // i18n-ignore
    })
  })

  it('allows selecting theme via theme dropdown', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tmdb_api_key: '',
      tmdb_configured: false,
    })

    render(<AdminSettings />)

    await waitFor(() => {
      expect(screen.getByTestId('theme-dropdown')).not.toBeNull()
    })

    fireEvent.click(screen.getByTestId('theme-dropdown'))

    const sunsetOption = screen.getByRole('option', { name: /Sunset Coral/ })
    fireEvent.click(sunsetOption)

    expect(screen.getByTestId('theme-dropdown').textContent).toContain('Sunset Coral')
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
      expect(screen.getByTestId('metadata-language-dropdown')).not.toBeNull()
    })

    fireEvent.click(screen.getByTestId('metadata-language-dropdown'))

    const englishOption = screen.getByRole('option', { name: /English/ })
    fireEvent.click(englishOption)

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
        enabled_watch_providers: 'prime,disney,apple,max,canal,crunchyroll,paramount,adn',
      })

    render(<AdminSettings />)

    await waitFor(() => {
      expect(screen.getByTestId('watch-providers-dropdown')).not.toBeNull()
    })

    fireEvent.click(screen.getByTestId('watch-providers-dropdown'))

    const netflixOption = screen.getByRole('option', { name: /Netflix/ })
    fireEvent.click(netflixOption)

    const saveBtn = screen.getByText('Save Settings')
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/admin/system-settings', expect.objectContaining({
        method: 'PUT',
        body: expect.stringContaining('enabled_watch_providers'),
      }))
    })
  })

  it('allows selecting all and deselecting all watch providers', async () => {
    vi.mocked(apiFetch).mockResolvedValueOnce({
      tmdb_api_key: '',
      tmdb_configured: false,
      enabled_watch_providers: 'netflix',
    })

    render(<AdminSettings />)

    await waitFor(() => {
      expect(screen.getByTestId('watch-providers-dropdown')).not.toBeNull()
    })

    fireEvent.click(screen.getByTestId('watch-providers-dropdown'))

    const selectAllBtn = screen.getByText('Select all')
    fireEvent.click(selectAllBtn)

    const deselectAllBtn = screen.getByText('Deselect all')
    fireEvent.click(deselectAllBtn)
  })
})
