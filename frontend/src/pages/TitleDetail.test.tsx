import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { TitleDetail } from './TitleDetail'
import { useApi } from '../hooks/useApi'
import type { Title } from '../types'

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
  useTitleStore: () => ({
    invalidate: vi.fn(),
  }),
}))

vi.mock('../context/UndoContext', () => ({
  useUndo: () => ({
    showUndo: vi.fn(),
  }),
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

    expect(screen.getByText('Send to Sonarr')).toBeTruthy()
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
    expect(screen.getByText('Re-add to Sonarr')).toBeTruthy()
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

    const readdBtn = screen.getByText('Re-add to Sonarr')
    fireEvent.click(readdBtn)

    // Confirmation drawer is opened
    expect(screen.getByText('Re-add to Sonarr?')).toBeTruthy()
    expect(screen.getByText(/This series was previously deleted and excluded from Sonarr/i)).toBeTruthy()
  })
})
