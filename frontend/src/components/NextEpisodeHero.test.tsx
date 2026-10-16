import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { NextEpisodeHero } from './NextEpisodeHero'
import type { Title, TitleRelation } from '../types'

const useApiMock = vi.fn()
vi.mock('../hooks/useApi', () => ({
  useApi: (path: string | null) => useApiMock(path),
}))

const routeMock = vi.fn()
vi.mock('preact-router', () => ({
  route: (url: string) => routeMock(url),
}))

describe('NextEpisodeHero', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    useApiMock.mockReturnValue({ data: { saga_chronological_next_up: true }, loading: false, error: null })
  })

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

  it('renders nothing when title status is completed or dropped with no relations', () => {
    const completedTitle: Title = { ...baseTitle, status: 'completed' }
    const { container: c1 } = render(<NextEpisodeHero title={completedTitle} onEpisodeToggle={vi.fn()} />)
    expect(c1.firstChild).toBeNull()

    const droppedTitle: Title = { ...baseTitle, status: 'dropped' }
    const { container: c2 } = render(<NextEpisodeHero title={droppedTitle} onEpisodeToggle={vi.fn()} />)
    expect(c2.firstChild).toBeNull()
  })

  it('renders continue saga hero when completed title has next chronological relation', () => {
    const sequelRelation: TitleRelation = {
      id: 50,
      title_id: 1,
      external_id: 200,
      provider: 'anilist',
      relation_type: 'SEQUEL',
      format: 'TV',
      title: 'Sequel Season 2',
      year: 2025,
      sort_order: 1,
      matched_title_id: 2,
      matched_status: 'plan_to_watch',
      created_at: '2024-01-01',
      updated_at: '2024-01-01',
    }

    const completedTitle: Title = {
      ...baseTitle,
      status: 'completed',
      relations: [sequelRelation],
    }

    const { getByText } = render(<NextEpisodeHero title={completedTitle} onEpisodeToggle={vi.fn()} />)

    expect(getByText('Continue Saga')).not.toBeNull()
    expect(getByText('Sequel Season 2')).not.toBeNull()
    expect(getByText('2025')).not.toBeNull()

    const btn = getByText('View Next in Saga').closest('button')
    expect(btn).not.toBeNull()
    fireEvent.click(btn!)
    expect(routeMock).toHaveBeenCalledWith('/title/2')
  })

  it('renders add to watchlist button when next saga title is not yet in library', () => {
    const unimportedRelation: TitleRelation = {
      id: 51,
      title_id: 1,
      external_id: 300,
      provider: 'anilist',
      relation_type: 'SEQUEL',
      format: 'MOVIE',
      title: 'Sequel Movie',
      year: 2026,
      sort_order: 2,
      matched_title_id: null,
      matched_status: null,
      created_at: '2024-01-01',
      updated_at: '2024-01-01',
    }

    const completedTitle: Title = {
      ...baseTitle,
      status: 'completed',
      relations: [unimportedRelation],
    }

    const { getByText } = render(<NextEpisodeHero title={completedTitle} onEpisodeToggle={vi.fn()} />)

    expect(getByText('Continue Saga')).not.toBeNull()
    expect(getByText('Sequel Movie')).not.toBeNull()
    expect(getByText('Add to Watchlist')).not.toBeNull()
  })

  it('does not render saga hero when saga_chronological_next_up is disabled in settings', () => {
    useApiMock.mockReturnValue({ data: { saga_chronological_next_up: false }, loading: false, error: null })

    const sequelRelation: TitleRelation = {
      id: 50,
      title_id: 1,
      external_id: 200,
      provider: 'anilist',
      relation_type: 'SEQUEL',
      format: 'TV',
      title: 'Sequel Season 2',
      year: 2025,
      sort_order: 1,
      matched_title_id: 2,
      matched_status: 'plan_to_watch',
      created_at: '2024-01-01',
      updated_at: '2024-01-01',
    }

    const completedTitle: Title = {
      ...baseTitle,
      status: 'completed',
      relations: [sequelRelation],
    }

    const { container } = render(<NextEpisodeHero title={completedTitle} onEpisodeToggle={vi.fn()} />)
    expect(container.firstChild).toBeNull()
  })

  it('does not render saga hero when all relations are already completed', () => {
    const completedRelation: TitleRelation = {
      id: 50,
      title_id: 1,
      external_id: 200,
      provider: 'anilist',
      relation_type: 'SEQUEL',
      format: 'TV',
      title: 'Sequel Season 2',
      year: 2025,
      sort_order: 1,
      matched_title_id: 2,
      matched_status: 'completed',
      created_at: '2024-01-01',
      updated_at: '2024-01-01',
    }

    const completedTitle: Title = {
      ...baseTitle,
      status: 'completed',
      relations: [completedRelation],
    }

    const { container } = render(<NextEpisodeHero title={completedTitle} onEpisodeToggle={vi.fn()} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders nothing when the only unwatched episode is TBA', () => {
    const tbaTitle: Title = {
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
        {
          id: 11,
          title_id: 1,
          season_number: 2,
          total_episodes: 1,
          episodes: [
            { id: 102, season_id: 11, episode: 1, name: 'TBA', air_date: null, watched: false, first_watched_at: null, last_watched_at: null },
          ],
        },
      ],
    }

    const { container } = render(<NextEpisodeHero title={tbaTitle} onEpisodeToggle={vi.fn()} />)
    expect(container.firstChild).toBeNull()
  })

  it('does not render when remaining unwatched episode has a future air date (e.g. The Pitt)', () => {
    const futureTitle: Title = {
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
        {
          id: 11,
          title_id: 1,
          season_number: 2,
          total_episodes: 1,
          episodes: [
            { id: 102, season_id: 11, episode: 1, name: '7:00 A.M.', air_date: '2099-01-01', watched: false, first_watched_at: null, last_watched_at: null },
          ],
        },
      ],
    }

    const { container } = render(<NextEpisodeHero title={futureTitle} onEpisodeToggle={vi.fn()} />)
    expect(container.firstChild).toBeNull()
  })
})
