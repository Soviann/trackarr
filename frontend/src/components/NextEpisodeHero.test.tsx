import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { NextEpisodeHero } from './NextEpisodeHero'
import type { Title } from '../types'

describe('NextEpisodeHero', () => {
  afterEach(() => {
    cleanup()
  })

  const baseTitle: Title = {
    id: 1,
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
    tvdb_id: null,
    my_rating: null,
    status: 'watching',
    series_status: 'returning',
    match_status: 'confirmed',
    original_title: null,
    match_source: null,
    overview: 'Test synopsis',
    genres: null,
    runtime: 50,
    total_watch_minutes: 50,
    tmdb_rating: 8.5,
    credits: null,
    anilist_rating: null,
    release_date: '2024-01-01',
    created_at: '2024-01-01',
    updated_at: '2024-01-01',
    names: [{ id: 1, title_id: 1, name: 'Sample Show', language: 'en', is_primary: true }],
    seasons: [
      {
        id: 10,
        title_id: 1,
        season_number: 1,
        total_episodes: 2,
        episodes: [
          { id: 101, season_id: 10, episode: 1, name: 'Pilot', air_date: '2024-01-01', watched: true, first_watched_at: null, last_watched_at: null },
          { id: 102, season_id: 10, episode: 2, name: 'Second Episode', air_date: '2024-01-08', watched: false, first_watched_at: null, last_watched_at: null },
        ],
      },
    ],
  }

  it('renders next episode hero with episode code, name and binge estimate', () => {
    const onToggle = vi.fn()
    const { getByText } = render(<NextEpisodeHero title={baseTitle} onEpisodeToggle={onToggle} />)

    expect(getByText('S01E02')).not.toBeNull()
    expect(getByText('Second Episode')).not.toBeNull()
    expect(getByText(/Left ~50m/)).not.toBeNull()
    expect(getByText(/Mark S01E02 as watched/)).not.toBeNull()

    const btn = getByText(/Mark S01E02 as watched/).closest('button')
    expect(btn).not.toBeNull()
    fireEvent.click(btn!)
    expect(onToggle).toHaveBeenCalledWith(102)
  })

  it('renders nothing when all episodes in watching series are watched', () => {
    const caughtUpTitle: Title = {
      ...baseTitle,
      seasons: [
        {
          id: 10,
          title_id: 1,
          season_number: 1,
          total_episodes: 1,
          episodes: [
            { id: 101, season_id: 10, episode: 1, name: 'Pilot', air_date: '2024-01-01', watched: true, first_watched_at: null, last_watched_at: null },
          ],
        },
      ],
    }

    const { container } = render(<NextEpisodeHero title={caughtUpTitle} onEpisodeToggle={vi.fn()} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders movie in plan to watch with duration and quick mark button', () => {
    const movieTitle: Title = {
      ...baseTitle,
      type: 'movie',
      status: 'plan_to_watch',
      runtime: 125,
      seasons: [],
    }

    const onStatusChange = vi.fn()
    const { getByText } = render(
      <NextEpisodeHero title={movieTitle} onEpisodeToggle={vi.fn()} onStatusChange={onStatusChange} />
    )

    expect(getByText(/Movie watchlist/)).not.toBeNull()
    expect(getByText(/2h 05m/)).not.toBeNull()

    const btn = getByText('Mark movie as watched').closest('button')
    expect(btn).not.toBeNull()
    fireEvent.click(btn!)
    expect(onStatusChange).toHaveBeenCalledWith('completed')
  })
})
