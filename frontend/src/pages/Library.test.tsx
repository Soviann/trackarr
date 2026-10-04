import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { Library } from './Library'
import type { Title } from '../types'

const createMockTitle = (id: number, name: string): Title => ({
  id,
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
  sonarr_id: null,
  sonarr_deleted_at: null,
  total_watch_minutes: 0,
  my_rating: null,
  status: 'plan_to_watch',
  series_status: null,
  match_status: 'confirmed',
  original_title: name,
  names: [{ id: id, title_id: id, name, language: 'en', is_primary: true }],
  match_source: null,
  overview: null,
  genres: null,
  runtime: null,
  tmdb_rating: null,
  credits: null,
  anilist_rating: null,
  release_date: '2024-01-01',
  seasons: [],
  created_at: '',
  updated_at: '',
})

const mockTitles = [
  createMockTitle(1, 'Movie One'),
  createMockTitle(2, 'Movie Two'),
  createMockTitle(3, 'Movie Three'),
]

const mockStoreState = {
  titles: mockTitles,
  total: 3,
  hasMore: false,
  counts: null,
  filter: {},
  loading: false,
  loadingMore: false,
  error: null,
  fetchTitles: vi.fn(),
  loadMore: vi.fn(),
  invalidate: vi.fn(),
  sort: { field: 'updated_at', order: 'desc' },
}

vi.mock('../store', () => ({
  useTitleStore: (sel?: (s: typeof mockStoreState) => unknown) =>
    sel ? sel(mockStoreState) : mockStoreState,
}))

vi.mock('../api', () => ({
  apiFetch: vi.fn().mockResolvedValue([]),
}))

vi.mock('../hooks/useApi', () => ({
  useApi: vi.fn().mockReturnValue({ data: null }),
}))

vi.mock('../hooks/useScrollRestoration', () => ({
  useScrollRestoration: vi.fn(),
}))

describe('Library desktop features', () => {
  beforeEach(() => {
    mockStoreState.titles = mockTitles
    mockStoreState.total = 3
    mockStoreState.loading = false
  })

  afterEach(() => {
    cleanup()
  })

  it('renders all titles in grid', () => {
    const { getByText } = render(<Library />)
    expect(getByText('Movie One')).toBeDefined()
    expect(getByText('Movie Two')).toBeDefined()
    expect(getByText('Movie Three')).toBeDefined()
  })

  it('supports Shift+Click for range selection and Escape to exit select mode', () => {
    const { getByText, queryByText } = render(<Library />)

    // Regular click on title 1 (sets lastClickedIdRef)
    const card1 = getByText('Movie One').closest('a')!
    fireEvent.click(card1)

    // Shift+Click on title 3 triggers range select from title 1 to title 3
    const card3 = getByText('Movie Three').closest('a')!
    fireEvent.click(card3, { shiftKey: true })

    // Bulk selection bar should appear showing "3 of 3"
    expect(getByText(/3 of 3/)).toBeDefined()

    // Press Escape to cancel selection mode
    fireEvent.keyDown(window, { key: 'Escape' })

    // Bulk selection bar is dismissed
    expect(queryByText(/3 of 3/)).toBeNull()
  })
})
