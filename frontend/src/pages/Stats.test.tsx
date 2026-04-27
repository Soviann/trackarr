import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, waitFor } from '@testing-library/preact'
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

const baseStats: StatsResponse = {
  overview: { total_titles: 1, total_movies: 1, total_series: 0, total_anime: 0, episodes_watched: 1, completion_rate: 1, average_rating: 8 },
  ratings: { distribution: Array(11).fill(0), average_by_type: {}, insight: '' },
  breakdown: { by_status: {}, by_type: {} },
  fun_stats: [],
  year_summary: { titles_added: 0, episodes_watched: 0, completions: 0 },
  genres: [],
  streaks: { current: 0, best: 0 },
  total_watch_minutes: 60,
  watched_this_year: 0,
  avg_rating_this_year: 0,
  minutes_this_week: 0,
}

describe('Stats — Recent activity', () => {
  beforeEach(() => {
    apiFetchMock.mockReset()
    useApiMock.mockReset()
    useApiMock.mockReturnValue({ data: baseStats, loading: false, error: null, mutate: vi.fn(), setData: vi.fn() })
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
