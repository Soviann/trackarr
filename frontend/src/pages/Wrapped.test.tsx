import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/preact'
import type { WrappedResponse } from '../types'

const apiFetchMock = vi.fn()

vi.mock('../api', () => ({
  apiFetch: (...args: unknown[]) => apiFetchMock(...args),
  ApiError: class extends Error {},
}))

const sampleWrapped: WrappedResponse = {
  year: 2026,
  available_years: [2026, 2025],
  overview: {
    total_titles: 42,
    total_movies: 20,
    total_series: 15,
    total_anime: 7,
    episodes_watched: 310,
    completion_rate: 0.85,
    average_rating: 8.4,
  },
  total_watch_minutes: 12450,
  top_favorites: {
    movies: [
      { id: 1, title: 'Dune: Part Two', year: 2026, type: 'movie', is_anime: false, my_rating: 10, watch_count: 2 },
    ],
    series: [
      { id: 2, title: 'Shōgun', year: 2026, type: 'series', is_anime: false, my_rating: 9, watch_count: 10 },
    ],
    anime: [
      { id: 3, title: 'Solo Leveling', year: 2026, type: 'series', is_anime: true, my_rating: 8, watch_count: 12 },
    ],
  },
  top_releases: {
    movies: [
      { id: 1, title: 'Dune: Part Two', year: 2026, type: 'movie', is_anime: false, my_rating: 10, watch_count: 2 },
    ],
    series: [
      { id: 2, title: 'Shōgun', year: 2026, type: 'series', is_anime: false, my_rating: 9, watch_count: 10 },
    ],
    anime: [
      { id: 3, title: 'Solo Leveling', year: 2026, type: 'series', is_anime: true, my_rating: 8, watch_count: 12 },
    ],
  },
  rewatch_champion: {
    title: { id: 1, title: 'Dune: Part Two', year: 2026, type: 'movie', is_anime: false, watch_count: 3 },
    total_plays: 3,
    is_movie: true,
  },
  top_genres: [{ genre: 'Sci-Fi', count: 12 }, { genre: 'Drama', count: 9 }],
  top_actors: [{ name: 'Tom Hanks', count: 4 }],
  top_directors: [{ name: 'Christopher Nolan', count: 3 }],
  persona: {
    title: 'The Nocturnal Cinephile',
    summary: 'A thrilling year packed with epic science-fiction and intense dramas.',
    quote: 'One more movie before sunrise.',
    fun_facts: [
      '62% of your watching was done after 8 PM.',
      'Your biggest binge was 8 episodes in one day.',
    ],
    badges: ['Night Owl', 'Cinephile'],
  },
}

describe('Wrapped Page', () => {
  beforeEach(() => {
    apiFetchMock.mockReset()
  })

  afterEach(() => {
    cleanup()
  })

  it('renders overview slide on initial load and navigates through slides', async () => {
    apiFetchMock.mockResolvedValueOnce(sampleWrapped)
    const { Wrapped } = await import('./Wrapped')
    const { container } = render(<Wrapped />)

    await waitFor(() => {
      expect(screen.getByText('Trackarr Wrapped')).toBeTruthy()
      expect(screen.getByText('Your Year in Media')).toBeTruthy()
      expect(screen.getByText('42')).toBeTruthy()
      expect(screen.getByText('310')).toBeTruthy()
    })

    // Advance to Slide 2 (Favorites)
    fireEvent.click(screen.getByTestId('progress-1'))
    await waitFor(() => {
      expect(screen.getByText('Top Favorites')).toBeTruthy()
      expect(screen.getByText('Dune: Part Two')).toBeTruthy()
    })

    // Advance to Slide 3 (Releases)
    fireEvent.click(screen.getByTestId('progress-2'))
    await waitFor(() => {
      expect(screen.getByText(/Best Releases/)).toBeTruthy()
      expect(screen.getByText('Dune: Part Two')).toBeTruthy()
    })

    // Advance to Slide 4 (Rewatch)
    fireEvent.click(screen.getByTestId('progress-3'))
    await waitFor(() => {
      expect(screen.getByText('Rewatch Champion')).toBeTruthy()
      expect(screen.getByText('Dune: Part Two')).toBeTruthy()
    })

    // Advance to Slide 5 (Cast & Genres)
    fireEvent.click(screen.getByTestId('progress-4'))
    await waitFor(() => {
      expect(screen.getByText('Cast, Crew & Genres')).toBeTruthy()
      expect(screen.getByText('Tom Hanks')).toBeTruthy()
      expect(screen.getByText('Christopher Nolan')).toBeTruthy()
    })

    // Advance to Slide 6 (Persona)
    fireEvent.click(screen.getByTestId('progress-5'))
    await waitFor(() => {
      expect(screen.getByText('Your AI Viewing Persona')).toBeTruthy()
      expect(screen.getByText('The Nocturnal Cinephile')).toBeTruthy()
      expect(screen.getByText(/One more movie before sunrise/)).toBeTruthy()
      expect(screen.getByText('Night Owl')).toBeTruthy()
    })
  })

  it('handles empty state when no titles were watched in the year', async () => {
    apiFetchMock.mockResolvedValueOnce({
      year: 2026,
      available_years: [2026],
      overview: { total_titles: 0, total_movies: 0, total_series: 0, total_anime: 0, episodes_watched: 0, completion_rate: 0, average_rating: 0 },
      total_watch_minutes: 0,
      top_favorites: { movies: [], series: [], anime: [] },
      top_releases: { movies: [], series: [], anime: [] },
      top_genres: [],
      top_actors: [],
      top_directors: [],
      persona: { title: 'The Casual Viewer', summary: '', quote: '', fun_facts: [] },
    })

    const { Wrapped } = await import('./Wrapped')
    render(<Wrapped />)

    await waitFor(() => {
      expect(screen.getByText('Trackarr Wrapped')).toBeTruthy()
    })
  })
})
