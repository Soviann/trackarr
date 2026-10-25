import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup, waitFor } from '@testing-library/preact'
import { ComingUp } from './ComingUp'
import type { UpcomingTitle } from '../types'
import * as api from '../api'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

const mockRoute = vi.fn()
vi.mock('preact-router', () => ({
  route: (path: string) => mockRoute(path),
}))

const mockTitles: UpcomingTitle[] = [
  {
    id: 1,
    name: 'Show One',
    type: 'series',
    is_anime: false,
    cover_url: '/covers/1.jpg',
    next_air_date: '2026-10-25',
    next_air_episode: 'S2 E4',
    status: 'watching',
  },
  {
    id: 2,
    name: 'Anime One',
    type: 'series',
    is_anime: true,
    cover_url: '/covers/2.jpg',
    next_air_date: '2026-10-26',
    next_air_episode: 'S1 E4',
    status: 'watching',
  },
  {
    id: 3,
    name: 'Movie One',
    type: 'movie',
    is_anime: false,
    cover_url: '/covers/3.jpg',
    next_air_date: '2026-11-01',
    next_air_episode: null,
    status: 'plan_to_watch',
  },
]

describe('ComingUp Page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.apiFetch).mockResolvedValue(mockTitles)
  })

  afterEach(() => {
    cleanup()
  })

  it('fetches /titles/upcoming and renders all titles', async () => {
    const { getByText, getAllByRole } = render(<ComingUp />)

    await waitFor(() => {
      expect(getByText('Show One')).toBeTruthy()
      expect(getByText('Anime One')).toBeTruthy()
      expect(getByText('Movie One')).toBeTruthy()
    })

    expect(api.apiFetch).toHaveBeenCalledWith(
      '/titles/upcoming',
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )

    // Airing count displayed in header
    expect(getByText('3 airing soon')).toBeTruthy()
  })

  it('renders episode tags and formatted dates at a glance', async () => {
    const { getByText } = render(<ComingUp />)

    await waitFor(() => {
      // Sublabels contain episode and date
      expect(getByText(/S2 E4/)).toBeTruthy()
      expect(getByText(/S1 E4/)).toBeTruthy()
    })
  })

  it('filters titles by category chip', async () => {
    const { getByText, queryByText } = render(<ComingUp />)

    await waitFor(() => {
      expect(getByText('Show One')).toBeTruthy()
    })

    // Filter by anime
    fireEvent.click(getByText('⛩️ Anime'))
    expect(getByText('Anime One')).toBeTruthy()
    expect(queryByText('Show One')).toBeNull()
    expect(queryByText('Movie One')).toBeNull()
    expect(getByText('1 airing soon')).toBeTruthy()

    // Filter by movies
    fireEvent.click(getByText(/Movies/))
    expect(getByText('Movie One')).toBeTruthy()
    expect(queryByText('Show One')).toBeNull()
    expect(queryByText('Anime One')).toBeNull()

    // Reset to all
    fireEvent.click(getByText('All'))
    expect(getByText('Show One')).toBeTruthy()
    expect(getByText('Anime One')).toBeTruthy()
    expect(getByText('Movie One')).toBeTruthy()
  })

  it('navigates to calendar on calendar shortcut click', async () => {
    const { getByTitle } = render(<ComingUp />)

    await waitFor(() => {
      expect(getByTitle('Calendar')).toBeTruthy()
    })

    fireEvent.click(getByTitle('Calendar'))
    expect(mockRoute).toHaveBeenCalledWith('/calendar')
  })

  it('shows empty state when no upcoming titles exist', async () => {
    vi.mocked(api.apiFetch).mockResolvedValueOnce([])
    const { getByText } = render(<ComingUp />)

    await waitFor(() => {
      expect(getByText('No upcoming releases')).toBeTruthy()
    })
  })
})
