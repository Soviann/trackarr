import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, cleanup, fireEvent } from '@testing-library/preact'
import { PosterHoverCard } from './PosterHoverCard'
import type { Title } from '../types'

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

const baseTitle: Title = {
  id: 101,
  type: 'series',
  is_anime: false,
  year: 2024,
  cover_url: 'https://example.com/cover.jpg',
  accent_hex: '#3b82f6',
  imdb_id: 'tt1234567',
  simkl_id: null,
  simkl_slug: null,
  anilist_id: null,
  tmdb_id: 999,
  tvdb_id: null,
  my_rating: 9,
  status: 'watching',
  series_status: 'returning',
  match_status: 'confirmed',
  original_title: 'Original Title',
  match_source: 'tmdb',
  overview: 'A thrilling space opera across galaxies.',
  genres: ['Sci-Fi', 'Action'],
  runtime: 45,
  total_watch_minutes: 90,
  tmdb_rating: 8.4,
  credits: null,
  anilist_rating: null,
  release_date: '2024-01-01',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  names: [{ id: 1, title_id: 101, name: 'Galactic Odyssey', language: 'en', is_primary: true }],
  seasons: [],
  next_episode: {
    id: 501,
    season_id: 10,
    season_number: 2,
    episode: 3,
    name: 'Nebula Flare',
    air_date: '2024-03-15',
    is_tba: false,
  },
  watch_providers: [
    { id: 8, name: 'Netflix' },
  ],
}

const mockRect: DOMRect = {
  left: 200,
  top: 150,
  right: 360,
  bottom: 390,
  width: 160,
  height: 240,
  x: 200,
  y: 150,
  toJSON: () => {},
}

describe('PosterHoverCard', () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('renders nominal details: name, year, overview, ratings, next episode and providers', () => {
    const onClose = vi.fn()
    const { getByText } = render(
      <PosterHoverCard
        title={baseTitle}
        anchorRect={mockRect}
        onClose={onClose}
      />
    )

    expect(getByText('Galactic Odyssey')).toBeTruthy()
    expect(getByText('2024')).toBeTruthy()
    expect(getByText('A thrilling space opera across galaxies.')).toBeTruthy()
    expect(getByText('★ 8.4')).toBeTruthy()
    expect(getByText('★ 9')).toBeTruthy()
    expect(getByText(/S2E3 — Nebula Flare/)).toBeTruthy()
  })

  it('triggers onQuickMark when +1 button is clicked', () => {
    const onQuickMark = vi.fn()
    const onClose = vi.fn()
    const { getByRole } = render(
      <PosterHoverCard
        title={baseTitle}
        anchorRect={mockRect}
        onClose={onClose}
        onQuickMark={onQuickMark}
      />
    )

    const plusBtn = getByRole('button', { name: /Mark next episode/i })
    fireEvent.click(plusBtn)
    expect(onQuickMark).toHaveBeenCalledTimes(1)
  })

  it('handles boundary case with empty overview and no ratings gracefully', () => {
    const emptyTitle: Title = {
      ...baseTitle,
      overview: null,
      tmdb_rating: null,
      my_rating: null,
      next_episode: undefined,
      watch_providers: undefined,
    }
    const onClose = vi.fn()
    const { getByText, queryByRole } = render(
      <PosterHoverCard
        title={emptyTitle}
        anchorRect={mockRect}
        onClose={onClose}
      />
    )

    expect(getByText('Galactic Odyssey')).toBeTruthy()
    expect(queryByRole('button', { name: /Mark next episode/i })).toBeNull()
  })

  it('calls onClose when Escape key is pressed', () => {
    const onClose = vi.fn()
    render(
      <PosterHoverCard
        title={baseTitle}
        anchorRect={mockRect}
        onClose={onClose}
      />
    )

    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })
})
