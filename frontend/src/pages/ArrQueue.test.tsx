import { render, screen, waitFor } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach } from 'vitest'
import { ArrQueue } from './ArrQueue'
import { apiFetch } from '../api'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

describe('ArrQueue', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('renders queue items and populates dropdowns correctly', async () => {
    // Mock the responses for Promise.all
    vi.mocked(apiFetch).mockImplementation((path: string) => {
      if (path.startsWith('/arr/queue')) {
        return Promise.resolve({
          items: [
            { id: 1, type: 'movie', name: 'Test Movie', is_anime: false },
            { id: 2, type: 'series', name: 'Test Show', is_anime: false }
          ],
          has_more: false
        })
      }
      if (path === '/admin/arr') {
        return Promise.resolve({
          radarr_std_monitored: 'true',
          radarr_std_search: 'true',
          radarr_std_root_folder: '/movies',
          radarr_std_quality_profile: '1',
          sonarr_std_monitored: 'true',
          sonarr_std_search: 'true',
          sonarr_std_root_folder: '/tv',
          sonarr_std_quality_profile: '2',
        })
      }
      if (path === '/arr/radarr/rootfolder') {
        return Promise.resolve([{ id: 1, path: '/movies' }])
      }
      if (path === '/arr/radarr/qualityprofile') {
        return Promise.resolve([{ id: 1, name: 'HD - 1080p' }])
      }
      if (path === '/arr/sonarr/rootfolder') {
        return Promise.resolve([{ id: 2, path: '/tv' }])
      }
      if (path === '/arr/sonarr/qualityprofile') {
        return Promise.resolve([{ id: 2, name: 'Any' }])
      }
      return Promise.reject(new Error('Not found'))
    })

    render(<ArrQueue />)

    // Wait for the loading state to finish
    await waitFor(() => {
      expect(screen.getByText('Test Movie')).not.toBeNull()
    })

    // Check that both items are rendered
    expect(screen.getByText('Test Movie')).not.toBeNull()
    expect(screen.getByText('Test Show')).not.toBeNull()

    // Check that dropdowns have options
    const rootFolderOptions = screen.getAllByRole('option', { name: '/movies' })
    expect(rootFolderOptions).toHaveLength(1)

    const tvRootFolderOptions = screen.getAllByRole('option', { name: '/tv' })
    expect(tvRootFolderOptions).toHaveLength(1)

    const radarrQualityOptions = screen.getAllByRole('option', { name: 'HD - 1080p' })
    expect(radarrQualityOptions).toHaveLength(1)

    const sonarrQualityOptions = screen.getAllByRole('option', { name: 'Any' })
    expect(sonarrQualityOptions).toHaveLength(1)
  })

  it('handles empty queue', async () => {
    vi.mocked(apiFetch).mockImplementation((path: string) => {
      if (path.startsWith('/arr/queue')) return Promise.resolve({ items: [], has_more: false })
      if (path === '/admin/arr') return Promise.resolve({})
      return Promise.resolve([])
    })

    render(<ArrQueue />)

    await waitFor(() => {
      expect(screen.getByText('The queue is empty.')).not.toBeNull()
    })
  })
})
