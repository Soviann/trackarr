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
  total_watch_minutes: 60,
  watched_this_year: 0,
  avg_rating_this_year: 0,
  minutes_this_week: 0,
}

describe('Stats', () => {
  beforeEach(() => {
    apiFetchMock.mockReset()
    useApiMock.mockReset()
    useApiMock.mockImplementation((path: string | null) => {
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

  it('renders top actors and top directors sections', async () => {
    apiFetchMock.mockResolvedValueOnce([])
    const { Stats } = await import('./Stats')
    render(<Stats />)

    expect(screen.getByText('// TOP ACTORS')).toBeTruthy()
    expect(screen.getByText('Timothée Chalamet')).toBeTruthy() // i18n-ignore
    expect(screen.getByText('// TOP DIRECTORS')).toBeTruthy()
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
})
