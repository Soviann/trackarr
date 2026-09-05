import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, cleanup } from '@testing-library/preact'
import { TitleCard } from './TitleCard'
import type { Title } from '../types'

vi.mock('../store', () => ({
  useTitleStore: (_sel: (s: unknown) => unknown) =>
    (_sel as (s: { sort: { field: string } }) => unknown)({ sort: { field: 'name' } }),
}))

const baseTitle: Title = {
  id: 42,
  type: 'series',
  is_anime: false,
  year: 2024,
  cover_url: null,
  accent_hex: null,
  imdb_id: null,
  simkl_id: null,
  simkl_slug: null,
  anilist_id: null,
  tmdb_id: 123,
  tvdb_id: null,
  my_rating: null,
  status: 'watching',
  series_status: null,
  match_status: 'confirmed',
  original_title: 'Test Title',
  match_source: 'tmdb',
  overview: null,
  genres: null,
  runtime: null,
  total_watch_minutes: 0,
  tmdb_rating: null,
  credits: null,
  anilist_rating: null,
  release_date: null,
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  names: [{ id: 1, title_id: 42, name: 'Test Title', language: 'en', is_primary: true }],
  seasons: [
    {
      id: 5,
      title_id: 42,
      season_number: 1,
      total_episodes: 10,
      episode_count: 10,
      watched_count: 2,
      episodes: [],
    },
  ],
}

describe('TitleCard', () => {
  afterEach(() => {
    cleanup()
  })
  it('renders quick mark button for watching series with next episode', () => {
    const watchingSeries: Title = {
      ...baseTitle,
      status: 'watching',
      next_episode: {
        id: 10,
        season_id: 5,
        episode: 3,
        season_number: 1,
      },
    }
    const { getByText, getByLabelText } = render(<TitleCard title={watchingSeries} />)
    expect(getByText('E3')).not.toBeNull()
    expect(getByLabelText('Mark E3 as watched')).not.toBeNull()
  })

  it('does not render quick mark button for dropped series even with next episode', () => {
    const droppedSeries: Title = {
      ...baseTitle,
      status: 'dropped',
      next_episode: {
        id: 10,
        season_id: 5,
        episode: 3,
        season_number: 1,
      },
    }
    const { queryByText, queryByLabelText } = render(<TitleCard title={droppedSeries} />)
    expect(queryByText('E3')).toBeNull()
    expect(queryByLabelText('Mark E3 as watched')).toBeNull()
  })
})
