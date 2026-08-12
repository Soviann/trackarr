import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { ArrQueue } from './ArrQueue'
import { apiFetch } from '../api'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

describe('ArrQueue', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it('renders queue items and populates dropdowns correctly', async () => {
    // Mock the responses for Promise.all
    vi.mocked(apiFetch).mockImplementation((path: string) => {
      if (path.startsWith('/arr/queue')) {
        return Promise.resolve({
          items: [
            { id: 1, type: 'movie', name: 'Test Movie', is_anime: false, cover_url: 'movie_cover.webp' },
            { id: 2, type: 'series', name: 'Test Show', is_anime: false, cover_url: null }
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

    // Check cover image src
    const img = screen.getByAltText('Test Movie') as HTMLImageElement
    expect(img.src).toContain('/api/covers/movie_cover.webp')

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

  it('handles ignore button click', async () => {
    vi.mocked(apiFetch).mockImplementation((path: string, options?: RequestInit) => {
      if (path.startsWith('/arr/queue')) {
        return Promise.resolve({
          items: [{ id: 42, type: 'movie', name: 'Item To Ignore', is_anime: false, cover_url: null }],
          has_more: false
        })
      }
      if (path === '/titles/42' && options?.method === 'PATCH') {
        return Promise.resolve({ success: true })
      }
      return Promise.resolve([])
    })

    render(<ArrQueue />)

    await waitFor(() => {
      expect(screen.getByText('Item To Ignore')).not.toBeNull()
    })

    const ignoreBtn = screen.getByText('Ignore')
    fireEvent.click(ignoreBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/titles/42', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ arr_ignored: true })
      })
      expect(screen.queryByText('Item To Ignore')).toBeNull()
    })
  })

  it('handles push button click', async () => {
    vi.mocked(apiFetch).mockImplementation((path: string, options?: RequestInit) => {
      if (path.startsWith('/arr/queue') && !path.endsWith('/push')) {
        return Promise.resolve({
          items: [{ id: 10, type: 'movie', name: 'Movie To Push', is_anime: false, cover_url: null }],
          has_more: false
        })
      }
      if (path === '/admin/arr') return Promise.resolve({})
      if (path === '/arr/radarr/rootfolder') return Promise.resolve([{ id: 1, path: '/movies' }])
      if (path === '/arr/radarr/qualityprofile') return Promise.resolve([{ id: 1, name: 'HD - 1080p' }])
      if (path === '/arr/queue/10/push' && options?.method === 'POST') {
        return Promise.resolve({ success: true })
      }
      return Promise.resolve([])
    })

    render(<ArrQueue />)

    await waitFor(() => {
      expect(screen.getByText('Movie To Push')).not.toBeNull()
    })

    const rootSelects = screen.getAllByRole('combobox')
    // rootSelects[2] is root folder, rootSelects[3] is quality profile
    fireEvent.change(rootSelects[2], { target: { value: '/movies' } })
    fireEvent.change(rootSelects[3], { target: { value: '1' } })

    const pushBtn = screen.getByText('Push to Radarr')
    fireEvent.click(pushBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/arr/queue/10/push', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          monitored: true,
          search: false,
          root_folder: '/movies',
          quality_profile: 1
        })
      })
      expect(screen.queryByText('Movie To Push')).toBeNull()
    })
  })
})
