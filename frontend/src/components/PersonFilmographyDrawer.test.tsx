import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { PersonFilmographyDrawer } from './PersonFilmographyDrawer'
import type { PaginatedResponse, Title } from '../types'

const useApiMock = vi.fn()

vi.mock('../hooks/useApi', () => ({
  useApi: (path: string | null) => useApiMock(path),
}))

vi.mock('../store', () => ({
  useTitleStore: (_sel: (s: unknown) => unknown) =>
    (_sel as (s: { sort: { field: string } }) => unknown)({ sort: { field: 'name' } }),
}))

const sampleTitle: Title = {
  id: 42,
  type: 'movie',
  is_anime: false,
  year: 2024,
  cover_url: null,
  accent_hex: null,
  imdb_id: null,
  simkl_id: null,
  simkl_slug: null,
  anilist_id: null,
  tmdb_id: null,
  tvdb_id: null,
  my_rating: 9,
  status: 'completed',
  series_status: null,
  match_status: 'confirmed',
  original_title: 'Dune Part Two',
  match_source: null,
  overview: 'Epic sci-fi movie',
  genres: ['Sci-Fi', 'Adventure'],
  runtime: 166,
  total_watch_minutes: 166,
  tmdb_rating: 8.5,
  credits: null,
  anilist_rating: null,
  release_date: '2024-03-01',
  created_at: '2024-03-01T00:00:00Z',
  updated_at: '2024-03-01T00:00:00Z',
  names: [{ id: 1, title_id: 42, name: 'Dune Part Two', language: 'en', is_primary: true }],
  seasons: [],
}

describe('PersonFilmographyDrawer', () => {
  beforeEach(() => {
    useApiMock.mockReset()
  })

  afterEach(() => {
    cleanup()
  })

  it('renders nothing when closed', () => {
    useApiMock.mockReturnValue({ data: null, loading: false, error: null })
    const { container } = render(
      <PersonFilmographyDrawer
        open={false}
        personName="Denis Villeneuve"
        role="director"
        onClose={vi.fn()}
      />
    )
    expect(container.firstChild).toBeNull()
  })

  it('renders person name and fetched titles when open', async () => {
    const mockData: PaginatedResponse = {
      titles: [sampleTitle],
      total: 1,
      has_more: false,
    }
    useApiMock.mockReturnValue({ data: mockData, loading: false, error: null, mutate: vi.fn() })

    render(
      <PersonFilmographyDrawer
        open={true}
        personName="Denis Villeneuve"
        role="director"
        onClose={vi.fn()}
      />
    )

    expect(screen.getByText('Denis Villeneuve')).toBeTruthy()
    expect(screen.getByText('Dune Part Two')).toBeTruthy()
  })

  it('renders loading state when loading', () => {
    useApiMock.mockReturnValue({ data: null, loading: true, error: null })

    render(
      <PersonFilmographyDrawer
        open={true}
        personName="Timothée Chalamet"
        role="actor"
        onClose={vi.fn()}
      />
    )

    expect(screen.getByText('Timothée Chalamet')).toBeTruthy()
    expect(screen.getByText('Loading...')).toBeTruthy()
  })

  it('renders empty state when no titles are found', () => {
    const emptyData: PaginatedResponse = {
      titles: [],
      total: 0,
      has_more: false,
    }
    useApiMock.mockReturnValue({ data: emptyData, loading: false, error: null })

    render(
      <PersonFilmographyDrawer
        open={true}
        personName="Unknown Person"
        role="actor"
        onClose={vi.fn()}
      />
    )

    expect(screen.getByText('No titles found in library')).toBeTruthy()
  })

  it('calls onClose when clicking "View full filmography"', () => {
    const mockData: PaginatedResponse = {
      titles: [sampleTitle],
      total: 1,
      has_more: false,
    }
    useApiMock.mockReturnValue({ data: mockData, loading: false, error: null, mutate: vi.fn() })
    const onClose = vi.fn()

    render(
      <PersonFilmographyDrawer
        open={true}
        personName="Denis Villeneuve"
        role="director"
        onClose={onClose}
      />
    )

    const viewAllBtn = screen.getAllByText('View full filmography')[0]
    fireEvent.click(viewAllBtn)
    expect(onClose).toHaveBeenCalled()
  })
})
