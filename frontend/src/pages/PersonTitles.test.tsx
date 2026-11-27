import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor, cleanup } from '@testing-library/preact'
import { PersonTitles } from './PersonTitles'
import { setLocale } from '../i18n'
import type { PaginatedResponse, Title } from '../types'

const useApiMock = vi.fn()

vi.mock('../hooks/useApi', () => ({
  useApi: (path: string | null) => useApiMock(path),
}))

vi.mock('../hooks/useScrollRestoration', () => ({
  useScrollRestoration: vi.fn(),
}))

vi.mock('../store', () => ({
  useTitleStore: (sel: (s: { sort: { field: string } }) => unknown) => sel({ sort: { field: 'name' } }),
}))

const duneTitle: Title = {
  id: 1,
  type: 'movie',
  is_anime: false,
  year: 2021,
  cover_url: null,
  accent_hex: null,
  imdb_id: null,
  simkl_id: null,
  simkl_slug: null,
  anilist_id: null,
  tmdb_id: 438631,
  tvdb_id: null,
  my_rating: 9,
  status: 'completed',
  series_status: null,
  match_status: 'confirmed',
  original_title: 'Dune',
  match_source: null,
  overview: null,
  genres: ['Sci-Fi'],
  runtime: 155,
  total_watch_minutes: 155,
  tmdb_rating: 8.0,
  credits: null,
  anilist_rating: null,
  release_date: '2021-10-22',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
  names: [{ id: 1, title_id: 1, name: 'Dune', language: 'en', is_primary: true }],
  seasons: [],
}

describe('PersonTitles', () => {
  beforeEach(() => {
    setLocale('en')
    useApiMock.mockReset()
    window.history.replaceState({}, '', '/person/Denis%20Villeneuve')
  })

  afterEach(() => {
    setLocale('en')
    vi.restoreAllMocks()
    cleanup()
  })

  it('renders 5 role filter pills, defaults to All, and fetches person titles without role param', () => {
    const data: PaginatedResponse = { titles: [duneTitle], total: 1, has_more: false }
    useApiMock.mockReturnValue({ data, loading: false, error: null, mutate: vi.fn() })

    render(<PersonTitles name="Denis Villeneuve" />)

    expect(screen.getByText('Denis Villeneuve')).toBeTruthy()
    expect(useApiMock).toHaveBeenCalledWith('/titles?person=Denis%20Villeneuve&limit=200')
    expect(screen.getByText('Dune')).toBeTruthy()

    expect(screen.getByRole('button', { name: 'All' }).getAttribute('aria-pressed')).toBe('true')
    expect(screen.getByRole('button', { name: 'Actor' }).getAttribute('aria-pressed')).toBe('false')
    expect(screen.getByRole('button', { name: 'Director' }).getAttribute('aria-pressed')).toBe('false')
    expect(screen.getByRole('button', { name: 'Writer' }).getAttribute('aria-pressed')).toBe('false')
    expect(screen.getByRole('button', { name: 'Composer' }).getAttribute('aria-pressed')).toBe('false')
  })

  it('initializes selected role from Preact Router url prop', () => {
    useApiMock.mockReturnValue({
      data: { titles: [duneTitle], total: 1, has_more: false },
      loading: false,
      error: null,
      mutate: vi.fn(),
    })

    render(<PersonTitles name="Denis Villeneuve" url="/person/Denis%20Villeneuve?role=writer" />)

    expect(useApiMock).toHaveBeenCalledWith('/titles?person=Denis%20Villeneuve&role=writer&limit=200')
    expect(screen.getByRole('button', { name: 'Writer' }).getAttribute('aria-pressed')).toBe('true')
  })

  it('initializes selected role from window.location.search case-insensitively and falls back to All on invalid value', () => {
    window.history.replaceState({}, '', '/person/Hans%20Zimmer?role=COMPOSER')
    useApiMock.mockReturnValue({
      data: { titles: [duneTitle], total: 1, has_more: false },
      loading: false,
      error: null,
      mutate: vi.fn(),
    })

    render(<PersonTitles name="Hans Zimmer" />)
    expect(useApiMock).toHaveBeenCalledWith('/titles?person=Hans%20Zimmer&role=composer&limit=200')
    expect(screen.getByRole('button', { name: 'Composer' }).getAttribute('aria-pressed')).toBe('true')

    cleanup()
    useApiMock.mockClear()
    render(<PersonTitles name="Hans Zimmer" url="/person/Hans%20Zimmer?role=unknown" />)
    expect(useApiMock).toHaveBeenCalledWith('/titles?person=Hans%20Zimmer&limit=200')
    expect(screen.getByRole('button', { name: 'All' }).getAttribute('aria-pressed')).toBe('true')
  })

  it('switches role filter via replaceState without pushing history entries and omits role param for All', async () => {
    useApiMock.mockReturnValue({
      data: { titles: [duneTitle], total: 1, has_more: false },
      loading: false,
      error: null,
      mutate: vi.fn(),
    })
    const replaceSpy = vi.spyOn(window.history, 'replaceState')
    const pushSpy = vi.spyOn(window.history, 'pushState')

    render(<PersonTitles name="Denis Villeneuve" url="/person/Denis%20Villeneuve" />)

    fireEvent.click(screen.getByRole('button', { name: 'Director' }))
    await waitFor(() => {
      expect(useApiMock).toHaveBeenLastCalledWith('/titles?person=Denis%20Villeneuve&role=director&limit=200')
    })
    expect(replaceSpy).toHaveBeenCalledWith(window.history.state, '', '/person/Denis%20Villeneuve?role=director')
    expect(pushSpy).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'All' }))
    await waitFor(() => {
      expect(useApiMock).toHaveBeenLastCalledWith('/titles?person=Denis%20Villeneuve&limit=200')
    })
    expect(replaceSpy).toHaveBeenLastCalledWith(window.history.state, '', '/person/Denis%20Villeneuve')
  })

  it('displays localized empty message per selected role and handles API error retry', () => {
    const mutate = vi.fn()
    useApiMock.mockReturnValue({
      data: { titles: [], total: 0, has_more: false },
      loading: false,
      error: null,
      mutate,
    })

    render(<PersonTitles name="Denis Villeneuve" url="/person/Denis%20Villeneuve" />)
    expect(screen.getByText('No titles found.')).toBeTruthy()

    fireEvent.click(screen.getByRole('button', { name: 'Composer' }))
    expect(screen.getByText('No titles found for this role.')).toBeTruthy()

    cleanup()
    useApiMock.mockReturnValue({
      data: null,
      loading: false,
      error: 'Failed to load',
      mutate,
    })
    render(<PersonTitles name="Denis Villeneuve" />)
    expect(screen.getByRole('alert').textContent).toContain('Failed to load')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(mutate).toHaveBeenCalledTimes(1)
  })

  it('localizes role filter pills and empty message in French', () => {
    setLocale('fr')
    useApiMock.mockReturnValue({
      data: { titles: [], total: 0, has_more: false },
      loading: false,
      error: null,
      mutate: vi.fn(),
    })

    render(<PersonTitles name="Hans Zimmer" url="/person/Hans%20Zimmer?role=writer" />)

    expect(screen.getByRole('button', { name: 'Tous' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Acteur' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'Réalisateur' })).toBeTruthy() // i18n-ignore
    expect(screen.getByRole('button', { name: 'Scénariste' }).getAttribute('aria-pressed')).toBe('true') // i18n-ignore
    expect(screen.getByRole('button', { name: 'Compositeur' })).toBeTruthy()
    expect(screen.getByText('Aucun titre trouvé pour ce rôle.')).toBeTruthy() // i18n-ignore
  })
})
