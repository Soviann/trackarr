import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useState } from 'preact/hooks'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/preact'
import { TitleDetail } from './TitleDetail'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import type { Title } from '../types'

const mockInvalidate = vi.fn()

vi.mock('../hooks/useApi', () => ({
  useApi: vi.fn(),
}))

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('../hooks/useScrollRestoration', () => ({
  useScrollRestoration: vi.fn(),
}))

vi.mock('../store', () => ({
  useTitleStore: (selector?: (st: { invalidate: typeof mockInvalidate }) => unknown) => {
    const state = { invalidate: mockInvalidate }
    return selector ? selector(state) : state
  },
}))

vi.mock('../context/UndoContext', () => ({
  useUndo: () => ({
    showUndo: vi.fn(),
  }),
}))

vi.mock('../components/FranchiseRelationsSection', () => ({
  FranchiseRelationsSection: () => null,
}))

const mockNormalSeries: Title = {
  id: 10,
  type: 'series',
  is_anime: false,
  year: 2024,
  cover_url: null,
  accent_hex: null,
  imdb_id: null,
  simkl_id: null,
  simkl_slug: null,
  anilist_id: null,
  tmdb_id: null,
  tvdb_id: 123,
  sonarr_id: null,
  sonarr_deleted_at: null,
  total_watch_minutes: 0,
  my_rating: null,
  status: 'watching',
  series_status: 'returning',
  match_status: 'confirmed',
  original_title: 'Regular Series',
  match_source: null,
  overview: null,
  genres: null,
  runtime: null,
  tmdb_rating: null,
  credits: null,
  anilist_rating: null,
  release_date: null,
  created_at: '',
  updated_at: '',
  names: [{ id: 1, title_id: 10, name: 'Regular Series', language: 'en', is_primary: true }],
  seasons: [],
}

describe('TitleDetail - Sonarr Re-add Flow', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it('renders standard "Send to Sonarr" button when series has not been deleted', () => {
    vi.mocked(useApi).mockReturnValue({
      data: mockNormalSeries,
      loading: false,
      error: null,
      mutate: vi.fn(),
      setData: vi.fn(),
    })

    render(<TitleDetail id="10" />)

    expect(screen.getAllByText('Send to Sonarr').length).toBeGreaterThanOrEqual(1)
    expect(screen.queryByText('Deleted')).toBeNull()
    expect(screen.queryByText('Re-add to Sonarr')).toBeNull()
  })

  it('renders "Deleted" badge and "Re-add to Sonarr" when series has sonarr_deleted_at', () => {
    const deletedSeries: Title = {
      ...mockNormalSeries,
      status: 'dropped',
      sonarr_deleted_at: '2026-09-26T12:00:00Z',
    }

    vi.mocked(useApi).mockReturnValue({
      data: deletedSeries,
      loading: false,
      error: null,
      mutate: vi.fn(),
      setData: vi.fn(),
    })

    render(<TitleDetail id="10" />)

    expect(screen.getByText('Deleted')).toBeTruthy()
    expect(screen.getAllByText('Re-add to Sonarr').length).toBeGreaterThanOrEqual(1)
    expect(screen.queryByText('Send to Sonarr')).toBeNull()
  })

  it('opens confirmation drawer when "Re-add to Sonarr" is clicked, and confirms intention', () => {
    const deletedSeries: Title = {
      ...mockNormalSeries,
      status: 'dropped',
      sonarr_deleted_at: '2026-09-26T12:00:00Z',
    }

    vi.mocked(useApi).mockReturnValue({
      data: deletedSeries,
      loading: false,
      error: null,
      mutate: vi.fn(),
      setData: vi.fn(),
    })

    render(<TitleDetail id="10" />)

    const readdButtons = screen.getAllByText('Re-add to Sonarr')
    fireEvent.click(readdButtons[0])

    // Confirmation drawer is opened
    expect(screen.getByText('Re-add to Sonarr?')).toBeTruthy()
    expect(screen.getByText(/This series was previously deleted and excluded from Sonarr/i)).toBeTruthy()
  })

  it('renders rating card and next episode hero together in the action row without duplicate history buttons in sidebar', () => {
    vi.mocked(useApi).mockReturnValue({
      data: mockNormalSeries,
      loading: false,
      error: null,
      mutate: vi.fn(),
      setData: vi.fn(),
    })

    const { container } = render(<TitleDetail id="10" />)

    // Rate strip exists
    expect(screen.getByText('RATE THIS TITLE:')).toBeTruthy()

    // History is only present in the hub / quick links, not duplicated in the desktop toolbar
    const desktopToolbar = container.querySelector('.desktopActionToolbar')
    if (desktopToolbar) {
      expect(desktopToolbar.textContent).not.toContain('Watch History')
    }
  })

  it('renders SeasonAniListStrip and empty episodes message for anime series without seasons', () => {
    const emptyAnimeSeries: Title = {
      ...mockNormalSeries,
      is_anime: true,
      seasons: [],
    }

    vi.mocked(useApi).mockReturnValue({
      data: emptyAnimeSeries,
      loading: false,
      error: null,
      mutate: vi.fn(),
      setData: vi.fn(),
    })

    render(<TitleDetail id="10" />)

    // SeasonAniListStrip is rendered with link button
    expect(screen.getByText('ANILIST')).toBeTruthy()
    expect(screen.getByText('Link entry')).toBeTruthy()

    // Empty episodes state is rendered
    expect(screen.getByText('No episodes announced yet')).toBeTruthy()
  })

  it('renders empty episodes message without SeasonAniListStrip for non-anime series without seasons', () => {
    const emptyNonAnimeSeries: Title = {
      ...mockNormalSeries,
      is_anime: false,
      seasons: [],
    }

    vi.mocked(useApi).mockReturnValue({
      data: emptyNonAnimeSeries,
      loading: false,
      error: null,
      mutate: vi.fn(),
      setData: vi.fn(),
    })

    render(<TitleDetail id="10" />)

    expect(screen.queryByText('ANILIST')).toBeNull()
    expect(screen.getByText('No episodes announced yet')).toBeTruthy()
  })

  it('renders empty episodes message when season exists but has 0 episodes', () => {
    const seriesWithEmptySeason: Title = {
      ...mockNormalSeries,
      seasons: [
        {
          id: 101,
          title_id: 10,
          season_number: 1,
          total_episodes: null,
          episode_count: 0,
          watched_count: 0,
          episodes: [],
        },
      ],
    }

    vi.mocked(useApi).mockReturnValue({
      data: seriesWithEmptySeason,
      loading: false,
      error: null,
      mutate: vi.fn(),
      setData: vi.fn(),
    })

    render(<TitleDetail id="10" />)

    expect(screen.getByText('No episodes announced yet')).toBeTruthy()
  })

  describe('Studio Bento & Keyboard Shortcuts', () => {
    const multiSeasonSeries: Title = {
      ...mockNormalSeries,
      seasons: [
        {
          id: 1,
          title_id: 10,
          season_number: 1,
          total_episodes: 2,
          episode_count: 2,
          watched_count: 1,
          episodes: [
            { id: 101, season_id: 1, episode: 1, name: 'Pilot', watched: true, air_date: null, first_watched_at: null, last_watched_at: null },
            { id: 102, season_id: 1, episode: 2, name: 'Second', watched: false, air_date: null, first_watched_at: null, last_watched_at: null },
          ],
        },
        {
          id: 2,
          title_id: 10,
          season_number: 2,
          total_episodes: 2,
          episode_count: 2,
          watched_count: 0,
          episodes: [
            { id: 201, season_id: 2, episode: 1, name: 'S2 Premiere', watched: false, air_date: null, first_watched_at: null, last_watched_at: null },
            { id: 202, season_id: 2, episode: 2, name: 'S2 Finale', watched: false, air_date: null, first_watched_at: null, last_watched_at: null },
          ],
        },
      ],
      next_episode: {
        id: 102,
        season_id: 1,
        season_number: 1,
        episode: 2,
        name: 'Second',
        air_date: '2024-01-08',
        is_tba: false,
      },
    }

    it('renders Bento Studio with season items and episodes', () => {
      vi.mocked(useApi).mockReturnValue({
        data: multiSeasonSeries,
        loading: false,
        error: null,
        mutate: vi.fn(),
        setData: vi.fn(),
      })

      render(<TitleDetail id="10" />)

      // Season items in Bento rail
      expect(screen.getByRole('button', { name: /S1 ·/ })).toBeTruthy()
      expect(screen.getByRole('button', { name: /S2 ·/ })).toBeTruthy()

      // Current season episodes
      expect(screen.getByText('— Pilot')).toBeTruthy()
      expect(screen.getByText('— Second')).toBeTruthy()
    })

    it('triggers next episode toggle when W key is pressed', () => {
      vi.mocked(apiFetch).mockClear()
      vi.mocked(useApi).mockReturnValue({
        data: multiSeasonSeries,
        loading: false,
        error: null,
        mutate: vi.fn(),
        setData: vi.fn(),
      })

      render(<TitleDetail id="10" />)

      fireEvent.keyDown(window, { key: 'w' })
      expect(apiFetch).toHaveBeenCalledWith('/titles/10/episodes/102', { method: 'PATCH' })
    })

    it('switches active season when number key 2 is pressed', () => {
      vi.mocked(useApi).mockReturnValue({
        data: multiSeasonSeries,
        loading: false,
        error: null,
        mutate: vi.fn(),
        setData: vi.fn(),
      })

      render(<TitleDetail id="10" />)

      // Initially on Season 1
      expect(screen.getByText('— Pilot')).toBeTruthy()

      // Press '2'
      fireEvent.keyDown(window, { key: '2' })

      // Now showing Season 2 episodes
      expect(screen.getByText('— S2 Premiere')).toBeTruthy()
      expect(screen.getByText('— S2 Finale')).toBeTruthy()
    })

    it('ignores shortcuts when user is focused inside an input element', () => {
      vi.mocked(useApi).mockReturnValue({
        data: multiSeasonSeries,
        loading: false,
        error: null,
        mutate: vi.fn(),
        setData: vi.fn(),
      })

      render(<TitleDetail id="10" />)
      vi.mocked(apiFetch).mockClear()

      const input = document.createElement('input')
      document.body.appendChild(input)
      input.focus()

      fireEvent.keyDown(input, { key: 'w' })
      expect(apiFetch).not.toHaveBeenCalled()

      document.body.removeChild(input)
    })
  })

  describe('Ambient Glow Mesh & Dynamic Styling', () => {
    it('sets --cover-accent, --cover-accent-wash, --cover-accent-glow and renders ambientMesh layer when accent_hex is present', () => {
      const accentedSeries: Title = {
        ...mockNormalSeries,
        accent_hex: '#10b981',
      }
      vi.mocked(useApi).mockReturnValue({
        data: accentedSeries,
        loading: false,
        error: null,
        mutate: vi.fn(),
        setData: vi.fn(),
      })

      const { container } = render(<TitleDetail id="10" />)

      const pageEl = container.querySelector('[class*="page"]') as HTMLElement
      expect(pageEl).toBeTruthy()
      expect(pageEl.style.getPropertyValue('--cover-accent')).toBe('#10b981')
      expect(pageEl.style.getPropertyValue('--cover-accent-wash')).toBe('rgba(16, 185, 129, 0.12)')
      expect(pageEl.style.getPropertyValue('--cover-accent-glow')).toBe('rgba(16, 185, 129, 0.24)')

      const ambientMesh = container.querySelector('[class*="ambientMesh"]')
      expect(ambientMesh).toBeTruthy()
    })
  })

  describe('Radarr deletion wiring', () => {
    it('clears radarr_id and downloaded_at, preserves status and rating, and calls invalidate on delete', async () => {
      const initialMovie: Title = {
        ...mockNormalSeries,
        id: 55,
        type: 'movie',
        tmdb_id: 603,
        tvdb_id: null,
        radarr_id: 777,
        downloaded_at: '2026-01-15T10:00:00Z',
        status: 'watching',
        my_rating: 9,
        series_status: null,
        original_title: 'The Matrix',
        names: [{ id: 1, title_id: 55, name: 'The Matrix', language: 'en', is_primary: true }],
      }

      vi.mocked(apiFetch).mockImplementation(async (path: string, init?: RequestInit) => {
        if (path === '/settings') return {}
        if (path === '/admin/arr') return { radarr_std_root_folder: '/movies', radarr_std_quality_profile: '1' }
        if (path.includes('/rootfolder')) return [{ id: 1, path: '/movies' }]
        if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD' }]
        if (path === '/arr/title/55' && (!init || !init.method || init.method === 'GET')) {
          return {
            app: 'radarr',
            exists: true,
            arr_id: 777,
            monitored: true,
            quality_profile_id: 1,
            root_folder_path: '/movies',
            has_file: true,
          }
        }
        if (path === '/arr/title/55' && init?.method === 'DELETE') {
          return { status: 'ok' }
        }
        return {}
      })

      function StatefulTitleDetail() {
        const [movie, setMovie] = useState<Title | null>(initialMovie)
        vi.mocked(useApi).mockImplementation((endpoint: string | null) => {
          if (endpoint === '/settings') {
            return { data: {}, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
          }
          return {
            data: movie,
            loading: false,
            error: null,
            mutate: vi.fn(),
            setData: (updater: unknown) => {
              setMovie((prev) => (typeof updater === 'function' ? (updater as (p: Title | null) => Title | null)(prev) : (updater as Title | null)))
            },
          }
        })
        return <TitleDetail id="55" />
      }

      render(<StatefulTitleDetail />)

      expect(screen.getAllByText('Manage in Radarr').length).toBeGreaterThanOrEqual(1)
      expect(screen.getByText(/Available/)).toBeTruthy()
      expect(screen.getByText('Watching')).toBeTruthy()
      expect(screen.getByText('9')).toBeTruthy()

      fireEvent.click(screen.getAllByText('Manage in Radarr')[0])

      const deleteBtn = await screen.findByRole('button', { name: /Delete from Radarr/i })
      fireEvent.click(deleteBtn)
      fireEvent.click(screen.getByRole('button', { name: /Confirm permanent deletion/i }))

      await waitFor(() => {
        expect(mockInvalidate).toHaveBeenCalledOnce()
        expect(screen.getAllByText('Send to Radarr').length).toBeGreaterThanOrEqual(1)
      })
      expect(screen.queryByText('Manage in Radarr')).toBeNull()
      expect(screen.queryByText(/Available/)).toBeNull()
      expect(screen.getByText('Watching')).toBeTruthy()
      expect(screen.getByText('9')).toBeTruthy()
    })

    it('keeps movie state intact and does not call invalidate when Radarr deletion fails', async () => {
      const initialMovie: Title = {
        ...mockNormalSeries,
        id: 56,
        type: 'movie',
        tmdb_id: 604,
        tvdb_id: null,
        radarr_id: 778,
        downloaded_at: '2026-01-15T10:00:00Z',
        status: 'watching',
        my_rating: 8,
        series_status: null,
        original_title: 'The Matrix Reloaded',
        names: [{ id: 1, title_id: 56, name: 'The Matrix Reloaded', language: 'en', is_primary: true }],
      }

      const setDataMock = vi.fn()
      vi.mocked(useApi).mockImplementation((endpoint: string | null) => {
        if (endpoint === '/settings') {
          return { data: {}, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
        }
        return { data: initialMovie, loading: false, error: null, mutate: vi.fn(), setData: setDataMock }
      })

      vi.mocked(apiFetch).mockImplementation(async (path: string, init?: RequestInit) => {
        if (path === '/admin/arr') return { radarr_std_root_folder: '/movies', radarr_std_quality_profile: '1' }
        if (path.includes('/rootfolder')) return [{ id: 1, path: '/movies' }]
        if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD' }]
        if (path === '/arr/title/56' && (!init || !init.method || init.method === 'GET')) {
          return {
            app: 'radarr',
            exists: true,
            arr_id: 778,
            monitored: true,
            quality_profile_id: 1,
            root_folder_path: '/movies',
            has_file: true,
          }
        }
        if (path === '/arr/title/56' && init?.method === 'DELETE') {
          throw new Error('Radarr error 500')
        }
        return {}
      })

      render(<TitleDetail id="56" />)
      fireEvent.click(screen.getAllByText('Manage in Radarr')[0])

      const deleteBtn = await screen.findByRole('button', { name: /Delete from Radarr/i })
      fireEvent.click(deleteBtn)
      fireEvent.click(screen.getByRole('button', { name: /Confirm permanent deletion/i }))

      await waitFor(() => {
        expect(screen.getByRole('alert').textContent).toContain('Radarr error 500')
      })
      expect(setDataMock).not.toHaveBeenCalled()
      expect(mockInvalidate).not.toHaveBeenCalled()
    })
  })

  describe('auto_open_imdb_on_rate setting', () => {
    const unratedMovie: Title = {
      ...mockNormalSeries,
      id: 77,
      type: 'movie',
      imdb_id: 'tt1375666',
      tmdb_id: 27205,
      tvdb_id: null,
      my_rating: null,
      series_status: null,
      original_title: 'Inception',
      names: [{ id: 1, title_id: 77, name: 'Inception', language: 'en', is_primary: true }],
    }

    it('opens IMDb automatically on inline rate when auto_open_imdb_on_rate is true', async () => {
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      vi.mocked(useApi).mockImplementation((endpoint: string | null) => {
        if (endpoint === '/settings') {
          return { data: { auto_open_imdb_on_rate: true }, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
        }
        return { data: unratedMovie, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
      })
      vi.mocked(apiFetch).mockResolvedValue({ ...unratedMovie, my_rating: 9 })

      render(<TitleDetail id="77" />)
      fireEvent.click(screen.getByRole('button', { name: 'Rate 9/10' }))

      await waitFor(() => {
        expect(apiFetch).toHaveBeenCalledWith('/titles/77', expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ my_rating: 9 }),
        }))
      })
      expect(openSpy).toHaveBeenCalledTimes(1)
      expect(openSpy).toHaveBeenCalledWith('https://www.imdb.com/title/tt1375666/', '_blank', 'noopener,noreferrer')
      openSpy.mockRestore()
    })

    it('does not open IMDb on inline rate when auto_open_imdb_on_rate is false', async () => {
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      vi.mocked(useApi).mockImplementation((endpoint: string | null) => {
        if (endpoint === '/settings') {
          return { data: { auto_open_imdb_on_rate: false }, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
        }
        return { data: unratedMovie, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
      })
      vi.mocked(apiFetch).mockResolvedValue({ ...unratedMovie, my_rating: 9 })

      render(<TitleDetail id="77" />)
      fireEvent.click(screen.getByRole('button', { name: 'Rate 9/10' }))

      await waitFor(() => {
        expect(apiFetch).toHaveBeenCalledWith('/titles/77', expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ my_rating: 9 }),
        }))
      })
      expect(openSpy).not.toHaveBeenCalled()
      openSpy.mockRestore()
    })

    it('keeps Save & rate working in Edit rating drawer when auto_open_imdb_on_rate is false', async () => {
      const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)
      const ratedMovie: Title = { ...unratedMovie, my_rating: 8 }
      vi.mocked(useApi).mockImplementation((endpoint: string | null) => {
        if (endpoint === '/settings') {
          return { data: { auto_open_imdb_on_rate: false }, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
        }
        return { data: ratedMovie, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
      })
      vi.mocked(apiFetch).mockResolvedValue({ ...ratedMovie, my_rating: 9 })

      const { container } = render(<TitleDetail id="77" />)
      const editRatingBtn = container.querySelector('button.btnEditRating')
      expect(editRatingBtn).not.toBeNull()
      fireEvent.click(editRatingBtn!)
      fireEvent.click(await screen.findByRole('button', { name: 'Rate 9/10' }))
      fireEvent.click(screen.getByText('Save & rate'))

      await waitFor(() => {
        expect(apiFetch).toHaveBeenCalledWith('/titles/77', expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ my_rating: 9 }),
        }))
      })
      expect(openSpy).toHaveBeenCalledTimes(1)
      expect(openSpy).toHaveBeenCalledWith('https://www.imdb.com/title/tt1375666/', '_blank', 'noopener,noreferrer')
      openSpy.mockRestore()
    })
  })
})

