import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { route } from 'preact-router'
import { PersonFilmographyDrawer } from './PersonFilmographyDrawer'
import { setLocale } from '../i18n'
import type { PaginatedResponse, PersonRole, Title } from '../types'

const useApiMock = vi.fn()

vi.mock('preact-router', async () => {
  const actual = await vi.importActual<typeof import('preact-router')>('preact-router')
  return {
    ...actual,
    route: vi.fn(),
  }
})

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
    setLocale('en')
    useApiMock.mockReset()
    vi.mocked(route).mockReset()
  })

  afterEach(() => {
    setLocale('en')
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
        personName="Timothée Chalamet" // i18n-ignore
        role="actor"
        onClose={vi.fn()}
      />
    )

    expect(screen.getByText('Timothée Chalamet')).toBeTruthy() // i18n-ignore
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

  it('calls onClose and routes to /person/:name?role=... when clicking "View full filmography"', () => {
    const mockData: PaginatedResponse = {
      titles: [sampleTitle],
      total: 1,
      has_more: false,
    }
    useApiMock.mockReturnValue({ data: mockData, loading: false, error: null, mutate: vi.fn() })
    const onClose = vi.fn()

    const { container } = render(
      <PersonFilmographyDrawer
        open={true}
        personName="Denis Villeneuve"
        role="director"
        onClose={onClose}
      />
    )

    expect(useApiMock).toHaveBeenCalledWith('/titles?person=Denis%20Villeneuve&role=director&limit=100')
    const viewAllLink = container.querySelector('a.viewAllBtn') as HTMLAnchorElement | null
    expect(viewAllLink?.getAttribute('href')).toBe('/person/Denis%20Villeneuve?role=director')
    fireEvent.click(viewAllLink!)
    expect(onClose).toHaveBeenCalled()
    expect(route).toHaveBeenCalledWith('/person/Denis%20Villeneuve?role=director')
  })

  it.each<{ role: PersonRole; icon: string; label: string; frLabel: string }>([
    { role: 'actor', icon: '🎬', label: 'Actor', frLabel: 'Acteur' },
    { role: 'director', icon: '🎥', label: 'Director', frLabel: 'Réalisateur' }, // i18n-ignore
    { role: 'writer', icon: '✍️', label: 'Writer', frLabel: 'Scénariste' }, // i18n-ignore
    { role: 'composer', icon: '🎵', label: 'Composer', frLabel: 'Compositeur' },
  ])('displays role icon, localized label, role-filtered API request, and person route for $role', ({ role, icon, label, frLabel }) => {
    const mockData: PaginatedResponse = {
      titles: [sampleTitle],
      total: 1,
      has_more: false,
    }
    useApiMock.mockReturnValue({ data: mockData, loading: false, error: null, mutate: vi.fn() })

    const { container } = render(
      <PersonFilmographyDrawer
        open={true}
        personName="Hans Zimmer"
        role={role}
        onClose={vi.fn()}
      />
    )

    expect(useApiMock).toHaveBeenCalledWith(`/titles?person=Hans%20Zimmer&role=${role}&limit=100`)
    expect(screen.getByText(icon)).toBeTruthy()
    expect(screen.getByText(new RegExp(`${label} · 1 title in library`))).toBeTruthy()
    const viewAllLink = container.querySelector('a.viewAllBtn') as HTMLAnchorElement | null
    expect(viewAllLink?.getAttribute('href')).toBe(`/person/Hans%20Zimmer?role=${role}`)

    cleanup()
    setLocale('fr')
    render(
      <PersonFilmographyDrawer
        open={true}
        personName="Hans Zimmer"
        role={role}
        onClose={vi.fn()}
      />
    )
    expect(screen.getByText(new RegExp(frLabel))).toBeTruthy()
  })

  it('omits role query parameter when role prop is not provided', () => {
    useApiMock.mockReturnValue({
      data: { titles: [], total: 0, has_more: false },
      loading: false,
      error: null,
      mutate: vi.fn(),
    })

    const { container } = render(
      <PersonFilmographyDrawer
        open={true}
        personName="Hans Zimmer"
        onClose={vi.fn()}
      />
    )

    expect(useApiMock).toHaveBeenCalledWith('/titles?person=Hans%20Zimmer&limit=100')
    expect(screen.getByText('Filmography')).toBeTruthy()
    const viewAllLink = container.querySelector('a.viewAllBtn') as HTMLAnchorElement | null
    expect(viewAllLink?.getAttribute('href')).toBe('/person/Hans%20Zimmer')
  })

  it('renders ErrorBanner and allows retry on API error', () => {
    const mutate = vi.fn()
    useApiMock.mockReturnValue({ data: null, loading: false, error: 'Server unavailable', mutate })

    render(
      <PersonFilmographyDrawer
        open={true}
        personName="Hans Zimmer"
        role="composer"
        onClose={vi.fn()}
      />
    )

    expect(screen.getByRole('alert').textContent).toContain('Server unavailable')
    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))
    expect(mutate).toHaveBeenCalledTimes(1)
  })
})
