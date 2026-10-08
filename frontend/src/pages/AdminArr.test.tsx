import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { AdminArr } from './AdminArr'
import { apiFetch } from '../api'
import { setLocale } from '../i18n'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

describe('AdminArr Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    localStorage.clear()
    setLocale('en')
  })

  afterEach(() => {
    cleanup()
  })

  const mockSettings = {
    radarr_url: 'http://192.168.1.50:7878',
    radarr_api_key: 'radarr-key-123',
    sonarr_url: 'http://192.168.1.50:8989',
    sonarr_api_key: 'sonarr-key-456',
    prowlarr_url: '',
    prowlarr_api_key: '',
    radarr_std_monitored: 'true',
    radarr_std_search: 'false',
    radarr_std_root_folder: '/movies',
    radarr_std_quality_profile: '1',
    radarr_anime_monitored: 'true',
    radarr_anime_search: 'false',
    radarr_anime_root_folder: '/anime-movies',
    radarr_anime_quality_profile: '2',
    sonarr_std_monitored: 'true',
    sonarr_std_search: 'false',
    sonarr_std_root_folder: '/tv',
    sonarr_std_quality_profile: '3',
    sonarr_anime_monitored: 'true',
    sonarr_anime_search: 'false',
    sonarr_anime_root_folder: '/anime-shows',
    sonarr_anime_quality_profile: '4',
  }

  const mockRootFolders = [{ id: 1, path: '/movies' }, { id: 2, path: '/anime-movies' }]
  const mockQualityProfiles = [{ id: 1, name: 'HD - 1080p' }, { id: 2, name: 'Anime 1080p' }]

  it('renders settings, connections, and defaults on nominal load', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/admin/arr') return mockSettings
      if (url === '/arr/radarr/rootfolder' || url === '/arr/sonarr/rootfolder') return mockRootFolders
      if (url === '/arr/radarr/qualityprofile' || url === '/arr/sonarr/qualityprofile') return mockQualityProfiles
      return {}
    })

    render(<AdminArr />)

    await waitFor(() => {
      expect(screen.getByText('Arr Stack')).not.toBeNull()
    })

    // Header and save button
    expect(screen.getByText('Save Settings')).not.toBeNull()

    // Connections card and statuses
    expect(screen.getByText('Connections')).not.toBeNull()
    expect(screen.getByText('Radarr (Movies)')).not.toBeNull()
    expect(screen.getByText('Sonarr (TV & Anime)')).not.toBeNull()
    expect(screen.getByText('Prowlarr (Release Indexers)')).not.toBeNull()

    const configuredBadges = screen.getAllByText('Configured')
    expect(configuredBadges.length).toBe(2) // Radarr and Sonarr
    expect(screen.getByText('Optional')).not.toBeNull() // Prowlarr

    // Defaults sections
    expect(screen.getByText('Radarr (Standard)')).not.toBeNull()
    expect(screen.getByText('Radarr (Anime)')).not.toBeNull()
    expect(screen.getByText('Sonarr (Standard)')).not.toBeNull()
    expect(screen.getByText('Sonarr (Anime)')).not.toBeNull()
  })

  it('handles testing connection successfully and re-fetching options', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string, opts?: RequestInit) => {
      if (url === '/admin/arr') return mockSettings
      if (url === '/arr/radarr/rootfolder') return mockRootFolders
      if (url === '/arr/radarr/qualityprofile') return mockQualityProfiles
      if (url === '/arr/sonarr/rootfolder') return mockRootFolders
      if (url === '/arr/sonarr/qualityprofile') return mockQualityProfiles
      if (url === '/admin/system-settings/test/radarr' && opts?.method === 'POST') {
        return { ok: true, message: 'Radarr connection successful (v5.0.3)' }
      }
      return {}
    })

    render(<AdminArr />)

    await waitFor(() => {
      expect(screen.getByText('Arr Stack')).not.toBeNull()
    })

    const testButtons = screen.getAllByText('Test')
    fireEvent.click(testButtons[0]) // First test button is Radarr

    await waitFor(() => {
      expect(screen.getByText(/Radarr connection successful/)).not.toBeNull()
    })
  })

  it('handles connection test failure gracefully', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string, opts?: RequestInit) => {
      if (url === '/admin/arr') return mockSettings
      if (url === '/arr/radarr/rootfolder' || url === '/arr/sonarr/rootfolder') return []
      if (url === '/arr/radarr/qualityprofile' || url === '/arr/sonarr/qualityprofile') return []
      if (url === '/admin/system-settings/test/sonarr' && opts?.method === 'POST') {
        return { ok: false, error: 'Connection refused: 192.168.1.50:8989' }
      }
      return {}
    })

    render(<AdminArr />)

    await waitFor(() => {
      expect(screen.getByText('Arr Stack')).not.toBeNull()
    })

    const testButtons = screen.getAllByText('Test')
    fireEvent.click(testButtons[1]) // Second test button is Sonarr

    await waitFor(() => {
      expect(screen.getByText(/Connection refused/)).not.toBeNull()
    })
  })

  it('allows editing settings and saves them via PUT /admin/arr', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string, opts?: RequestInit) => {
      if (url === '/admin/arr' && (!opts || !opts.method)) return mockSettings
      if (url === '/admin/arr' && opts?.method === 'PUT') return {}
      if (url.startsWith('/arr/')) return []
      return {}
    })

    render(<AdminArr />)

    await waitFor(() => {
      expect(screen.getByText('Arr Stack')).not.toBeNull()
    })

    // Modify Radarr URL
    const radarrUrlInput = screen.getByPlaceholderText('http://radarr:7878') as HTMLInputElement
    fireEvent.input(radarrUrlInput, { target: { value: 'http://radarr.local:7878' } })

    // Click Save
    const saveButton = screen.getByText('Save Settings')
    fireEvent.click(saveButton)

    await waitFor(() => {
      expect(screen.getByText('Settings saved successfully!')).not.toBeNull()
    })

    expect(apiFetch).toHaveBeenCalledWith(
      '/admin/arr',
      expect.objectContaining({
        method: 'PUT',
        body: expect.stringContaining('http://radarr.local:7878'),
      })
    )
  })

  it('displays error state when initial fetch fails', async () => {
    vi.mocked(apiFetch).mockImplementation(async (url: string) => {
      if (url === '/admin/arr') throw new Error('Database connection failed')
      return []
    })

    render(<AdminArr />)

    await waitFor(() => {
      expect(screen.getByText('Database connection failed')).not.toBeNull()
    })

    // Header remains visible
    expect(screen.getByText('Arr Stack')).not.toBeNull()
  })
})
