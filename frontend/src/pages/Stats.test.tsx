import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/preact'
import type { StatsResponse, ActivityEvent } from '../types'

const apiFetchMock = vi.fn()
const useApiMock = vi.fn()

vi.mock('../api', () => ({
  apiFetch: (...args: unknown[]) => apiFetchMock(...args),
  ApiError: class extends Error {},
}))

vi.mock('../hooks/useApi', () => ({
  useApi: (path: string | null) => useApiMock(path),
}))

vi.mock('../store', () => ({
  useTitleStore: (_sel: (s: unknown) => unknown) =>
    (_sel as (s: { sort: { field: string } }) => unknown)({ sort: { field: 'name' } }),
}))

const baseStats: StatsResponse = {
  overview: { total_titles: 1, total_movies: 1, total_series: 0, total_anime: 0, episodes_watched: 1, completion_rate: 1, average_rating: 8 },
  ratings: { distribution: Array(11).fill(0), average_by_type: {}, insight: '' },
  breakdown: { by_status: {}, by_type: {} },
  fun_stats: [],
  year_summary: { titles_added: 0, episodes_watched: 0, completions: 0 },
  genres: [{ genre: 'Action', count: 5 }],
  top_actors: [{ name: 'Timothée Chalamet', count: 3 }], // i18n-ignore
  top_directors: [{ name: 'Denis Villeneuve', count: 2 }],
  streaks: { current: 0, best: 0 },
  total_watch_minutes: 1703040, // 3 yrs 87 d 16 h (3*525600 + 87*1440 + 16*60)
  available_years: [2026, 2025, 2024],
  watched_this_year: 0,
  avg_rating_this_year: 0,
  minutes_this_week: 0,
}

describe('Stats', () => {
  beforeEach(() => {
    localStorage.clear()
    apiFetchMock.mockReset()
    useApiMock.mockReset()
    useApiMock.mockImplementation((path: string | null) => {
      if (path?.startsWith('/stats?')) {
        return { data: baseStats, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
      }
      if (path === '/stats') {
        return { data: baseStats, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
      }
      if (path?.startsWith('/titles?person=')) {
        return { data: { titles: [], total: 0, has_more: false }, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
      }
      return { data: null, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
    })
  })

  afterEach(() => {
    cleanup()
  })

  it('renders humanized watch time hero card and subtitle', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    render(<Stats />)

    expect(screen.getByText('Cumulative Watch Time')).toBeTruthy()
    expect(screen.getByText(/3 yrs 87 d 16 h|3 ans 87 j 16 h/)).toBeTruthy()
  })

  it('renders filter bar pills and updates query params on click', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    render(<Stats />)

    expect(screen.getByText('All history')).toBeTruthy()
    expect(screen.getByText('Last 30 days')).toBeTruthy()
    expect(screen.getByText('🎬 Movies')).toBeTruthy()
    expect(screen.getByText('📺 Series')).toBeTruthy()
    expect(screen.getByText('⛩️ Anime')).toBeTruthy()

    // Click on movies filter
    const moviesPill = screen.getByText('🎬 Movies')
    fireEvent.click(moviesPill)

    await waitFor(() => {
      expect(useApiMock).toHaveBeenCalledWith(expect.stringContaining('media_type=movie'))
    })
  })

  it('changing year select switches timeframe to year', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    render(<Stats />)

    const select = screen.getByLabelText('Filter by year')
    fireEvent.input(select, { target: { value: '2025' } })

    await waitFor(() => {
      expect(useApiMock).toHaveBeenCalledWith(expect.stringContaining('timeframe=year'))
      expect(useApiMock).toHaveBeenCalledWith(expect.stringContaining('year=2025'))
    })
  })

  it('renders top actors and top directors sections', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    render(<Stats />)

    expect(screen.getByText('Top Actors')).toBeTruthy()
    expect(screen.getByText('Timothée Chalamet')).toBeTruthy() // i18n-ignore
    expect(screen.getByText('Top Directors')).toBeTruthy()
    expect(screen.getByText('Denis Villeneuve')).toBeTruthy()
  })

  it('clicking an actor opens the person filmography drawer', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    render(<Stats />)

    const actorBtn = screen.getByText('Timothée Chalamet') // i18n-ignore
    fireEvent.click(actorBtn)

    // The drawer should open with the actor name in the header
    await waitFor(() => {
      const headings = screen.getAllByText('Timothée Chalamet') // i18n-ignore
      expect(headings.length).toBeGreaterThanOrEqual(2)
    })
  })

  it('activity row href uses singular SPA route /title/:id (not plural)', async () => {
    const event: ActivityEvent = {
      title_id: 7555,
      title_name: 'Some Title',
      cover_url: null,
      title_type: 'movie',
      episode_id: null,
      episode_name: null,
      season_number: null,
      episode_number: null,
      watched_at: '2026-04-27T12:00:00Z',
      is_completion: false,
    }
    apiFetchMock.mockResolvedValueOnce([event])

    const { Stats } = await import('./Stats')
    const { container } = render(<Stats />)

    const anchor = await waitFor(() => {
      const a = container.querySelector(`a[href="/title/${event.title_id}"]`)
      if (!a) throw new Error('activity anchor not yet rendered')
      return a
    })

    // Guard against the regression: SPA route is `/title/:id` (singular), NOT `/titles/:id`.
    expect(anchor.getAttribute('href')).toBe('/title/7555')
    expect(anchor.getAttribute('href')).not.toMatch(/^\/titles\//)
  })

  it('loadMore calls apiFetch with /stats/activity (no /api prefix)', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    render(<Stats />)

    await waitFor(() => expect(apiFetchMock).toHaveBeenCalled())
    const path = apiFetchMock.mock.calls[0][0] as string
    expect(path.startsWith('/stats/activity')).toBe(true)
    expect(path.startsWith('/api/')).toBe(false)
  })

  it('allows dismissing the wrapped banner and remembers in localStorage', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    const { container } = render(<Stats />)

    const dismissBtn = screen.getByLabelText('Dismiss banner')
    expect(dismissBtn).toBeTruthy()
    fireEvent.click(dismissBtn)

    // Banner should disappear
    expect(container.querySelector('[class*="wrappedBanner"]')).toBeNull()
    const currentYear = new Date().getFullYear()
    expect(localStorage.getItem(`trackarr:wrapped_banner_dismissed_${currentYear}`)).toBe('1')
  })

  it('filters out 0-title archives and does not render them', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    useApiMock.mockImplementation((path: string | null) => {
      if (path?.startsWith('/stats?')) {
        return { data: baseStats, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
      }
      if (path === '/stats/wrapped/archives') {
        return {
          data: [
            { year: 2024, persona_title: 'Ghost', total_titles: 0, total_watch_minutes: 0, created_at: '' },
          ],
          loading: false,
          error: null,
          mutate: vi.fn(),
          setData: vi.fn(),
        }
      }
      return { data: null, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
    })

    const { Stats } = await import('./Stats')
    const { container } = render(<Stats />)

    // Since total_titles is 0, the section should not render
    expect(screen.queryByText('Past Wrapped Archives')).toBeNull()
    expect(container.querySelector('[class*="archiveCard"]')).toBeNull()
  })

  it('collapses and expands valid archives', async () => {
    localStorage.removeItem('trackarr:wrapped_archives_collapsed')
    apiFetchMock.mockResolvedValueOnce([])
    useApiMock.mockImplementation((path: string | null) => {
      if (path?.startsWith('/stats?')) {
        return { data: baseStats, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
      }
      if (path === '/stats/wrapped/archives') {
        return {
          data: [
            { year: 2025, persona_title: 'Voyager', total_titles: 10, total_watch_minutes: 500, created_at: '' },
          ],
          loading: false,
          error: null,
          mutate: vi.fn(),
          setData: vi.fn(),
        }
      }
      return { data: null, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() }
    })

    const { Stats } = await import('./Stats')
    render(<Stats />)

    // Initially expanded
    expect(screen.getByText('Past Wrapped Archives')).toBeTruthy()
    expect(screen.getByText('Voyager')).toBeTruthy()

    // Click collapse
    const collapseBtn = screen.getByText(/Collapse/)
    fireEvent.click(collapseBtn)

    // Now compact mode
    expect(screen.queryByText('Past Wrapped Archives')).toBeNull()
    expect(screen.getByText(/Show cards \(1\)/)).toBeTruthy()
    expect(screen.getByRole('button', { name: '2025' })).toBeTruthy()

    // Click expand
    const expandBtn = screen.getByText(/Show cards \(1\)/)
    fireEvent.click(expandBtn)
    expect(screen.getByText('Past Wrapped Archives')).toBeTruthy()
  })

  it('renders semantic meter roles on ratings, genres, and person tracks', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    render(<Stats />)

    const meters = screen.getAllByRole('meter')
    expect(meters.length).toBeGreaterThan(0)

    // Check genre meter
    const actionMeter = screen.getByLabelText('Action: 5')
    expect(actionMeter).toBeTruthy()
    expect(actionMeter.getAttribute('aria-valuenow')).toBe('5')
    expect(actionMeter.getAttribute('aria-valuemin')).toBe('0')

    // Check actor meter
    const actorMeter = screen.getByLabelText('Timothée Chalamet: 3') // i18n-ignore
    expect(actorMeter).toBeTruthy()
    expect(actorMeter.getAttribute('aria-valuenow')).toBe('3')
  })

  it('renders wrapped banner as accessible link with separated dismiss button', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    const { container } = render(<Stats />)

    const bannerLink = container.querySelector('a[class*="wrappedBannerLink"]')
    expect(bannerLink).toBeTruthy()
    expect(bannerLink?.getAttribute('href')).toBe('/wrapped')

    const dismissBtn = screen.getByLabelText('Dismiss banner')
    expect(dismissBtn).toBeTruthy()
    // Dismiss button should not be a descendant of the banner anchor link
    expect(bannerLink?.contains(dismissBtn)).toBe(false)
  })
})

