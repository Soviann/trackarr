import { render, cleanup, fireEvent, waitFor } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import type { Title } from '../types'
import { apiFetch } from '../api'
import { Search } from './Search'

vi.mock('../api', () => ({
  apiFetch: vi.fn().mockResolvedValue({ titles: [], total: 0, has_more: false }),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

vi.mock('../hooks/useScrollRestoration', () => ({
  useScrollRestoration: vi.fn(),
}))

const mockIsDesktop = vi.fn().mockReturnValue(false)
vi.mock('../hooks/useIsDesktop', () => ({
  useIsDesktop: () => mockIsDesktop(),
}))

vi.mock('../context/UndoContext', () => ({
  useUndo: () => ({
    showUndo: vi.fn(),
  }),
}))

const mockSearchState = {
  query: '',
  results: [] as Title[],
  total: 0,
  hasMore: false,
  loading: false,
  loadingMore: false,
  error: null,
  searchOnTMDB: false,
  discoveryResults: [],
  loadingDiscovery: false,
  setDiscoveryResults: vi.fn(),
  setLoadingDiscovery: vi.fn(),
  search: vi.fn(),
  loadMore: vi.fn(),
  setQuery: vi.fn(),
  clear: vi.fn(),
  setSearchOnTMDB: vi.fn(),
}

const mockTitleState = {
  filter: {},
  invalidate: vi.fn(),
}

vi.mock('../store', () => {
  const useSearchStore = Object.assign(
    (selector: (s: typeof mockSearchState) => unknown) => selector(mockSearchState),
    {
      getState: () => mockSearchState,
    }
  )

  const useTitleStore = Object.assign(
    (selector: (s: typeof mockTitleState) => unknown) => selector(mockTitleState),
    {
      getState: () => mockTitleState,
    }
  )

  return { useSearchStore, useTitleStore }
})

describe('Search Page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.history.pushState({}, '', '/search')
  })

  afterEach(() => {
    cleanup()
    window.history.pushState({}, '', '/')
  })

  it('prefills search query with exact localized name when merge parameters are present', () => {
    window.history.pushState({}, '', '/search?mergeSourceId=42&mergeSourceName=Frieren%3A%20Beyond%20Journey%27s%20End')

    render(<Search />)

    expect(mockSearchState.setQuery).toHaveBeenCalledWith("Frieren: Beyond Journey's End")
  })

  it('does not prefill search query when merge parameters are absent', () => {
    window.history.pushState({}, '', '/search')

    render(<Search />)

    expect(mockSearchState.setQuery).not.toHaveBeenCalled()
  })

  it('does not prefill search query when mergeSourceId is present but mergeSourceName is absent', () => {
    window.history.pushState({}, '', '/search?mergeSourceId=42')

    render(<Search />)

    expect(mockSearchState.setQuery).not.toHaveBeenCalled()
  })

  it('mobile view: does not render desktopHeader', () => {
    mockIsDesktop.mockReturnValue(false)
    const { container } = render(<Search />)

    expect(container.querySelector('.desktopHeader')).toBeNull()
  })

  it('desktop view: renders desktopHeader with SearchBar and view switcher', () => {
    mockIsDesktop.mockReturnValue(true)
    const { container } = render(<Search />)

    expect(container.querySelector('.desktopHeader')).not.toBeNull()
    expect(container.querySelector('.viewSwitcher')).not.toBeNull()
  })

  it('renders search results as semantic links with href to title detail', () => {
    mockIsDesktop.mockReturnValue(false)
    mockSearchState.query = 'Frieren'
    mockSearchState.results = [
      {
        id: 42,
        type: 'series',
        is_anime: true,
        year: 2023,
        status: 'watching',
        names: [{ name: "Frieren: Beyond Journey's End", language: 'en', is_primary: true }],
      } as unknown as Title,
    ]

    const { container } = render(<Search />)

    const cardLink = container.querySelector('a.card') as HTMLAnchorElement | null
    expect(cardLink).not.toBeNull()
    expect(cardLink?.getAttribute('href')).toBe('/title/42')
  })

  it('deduplicates external discovery items matching local library results and displays badge', () => {
    mockIsDesktop.mockReturnValue(false)
    mockSearchState.query = 'Inception'
    mockSearchState.searchOnTMDB = true
    mockSearchState.results = [
      {
        id: 10,
        type: 'movie',
        tmdb_id: 501,
        year: 2010,
        status: 'watching',
        names: [{ name: 'Inception', language: 'en', is_primary: true }],
      } as unknown as Title,
    ]
    mockSearchState.discoveryResults = [
      {
        id: 'tmdb-501',
        tmdbId: 501,
        title: 'Inception',
        year: 2010,
        type: 'movie',
        isAnime: false,
        posterUrl: null,
        source: 'TMDB',
      },
      {
        id: 'tmdb-502',
        tmdbId: 502,
        title: 'Interstellar',
        year: 2014,
        type: 'movie',
        isAnime: false,
        posterUrl: null,
        source: 'TMDB',
      },
    ] as any

    const { container } = render(<Search />)

    // Local library section has Inception
    expect(container.querySelector('a.card[href="/title/10"]')).not.toBeNull()

    // Discovery section has Interstellar
    expect(container.querySelector('.discoveryTitle')?.textContent).toBe('Interstellar')

    // Discovery section does NOT have duplicate Inception card
    const discoveryCards = container.querySelectorAll('.discoveryCard')
    expect(discoveryCards.length).toBe(1)

    // Hidden in library badge is shown with count 1
    const hiddenBadge = container.querySelector('.hiddenBadge')
    expect(hiddenBadge).not.toBeNull()
    expect(hiddenBadge?.textContent).toContain('1 already in library')
  })

  it('displays informative notice when all external discovery results are already in the library', () => {
    mockIsDesktop.mockReturnValue(false)
    mockSearchState.query = 'Inception'
    mockSearchState.searchOnTMDB = true
    mockSearchState.results = [
      {
        id: 10,
        type: 'movie',
        tmdb_id: 501,
        year: 2010,
        status: 'watching',
        names: [{ name: 'Inception', language: 'en', is_primary: true }],
      } as unknown as Title,
    ]
    mockSearchState.discoveryResults = [
      {
        id: 'tmdb-501',
        tmdbId: 501,
        title: 'Inception',
        year: 2010,
        type: 'movie',
        isAnime: false,
        posterUrl: null,
        source: 'TMDB',
      },
    ] as any

    const { container } = render(<Search />)

    // No discovery cards displayed
    expect(container.querySelectorAll('.discoveryCard').length).toBe(0)

    // All external results in library notice is displayed
    const allHiddenNotice = container.querySelector('.allHiddenNotice')
    expect(allHiddenNotice).not.toBeNull()
    expect(allHiddenNotice?.textContent).toContain('All external results are already in your library')
  })

  it('adds discovery item and converts card to full-card link in collection', async () => {
    mockIsDesktop.mockReturnValue(true)
    mockSearchState.query = 'Survivor'
    mockSearchState.searchOnTMDB = true
    mockSearchState.results = []
    mockSearchState.discoveryResults = [
      {
        id: 'tmdb-999',
        tmdbId: 999,
        title: 'Survivor',
        year: 2000,
        type: 'series',
        isAnime: false,
        posterUrl: null,
        source: 'TMDB',
      },
    ] as any

    vi.mocked(apiFetch).mockResolvedValueOnce({
      id: 77,
      type: 'series',
      year: 2000,
      status: 'plan_to_watch',
      names: [{ name: 'Survivor', language: 'en', is_primary: true }],
    } as any)

    const { container } = render(<Search />)

    const addBtn = container.querySelector('.btnGridAction') as HTMLButtonElement
    expect(addBtn).not.toBeNull()
    fireEvent.click(addBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith(
        '/titles',
        expect.objectContaining({
          method: 'POST',
        })
      )
      expect(mockSearchState.setDiscoveryResults).toHaveBeenCalled()
    })
  })

  it('isolates ExternalLinks clicks and prevents card navigation', () => {
    mockIsDesktop.mockReturnValue(false)
    mockSearchState.query = 'Inception'
    mockSearchState.searchOnTMDB = true
    mockSearchState.results = []
    mockSearchState.discoveryResults = [
      {
        id: 'tmdb-501',
        tmdbId: 501,
        title: 'Inception',
        year: 2010,
        type: 'movie',
        isAnime: false,
        posterUrl: null,
        source: 'TMDB',
      },
    ] as any

    const { container } = render(<Search />)

    const tmdbBadge = container.querySelector('a[href="https://www.themoviedb.org/movie/501"]') as HTMLAnchorElement
    expect(tmdbBadge).not.toBeNull()

    const stopPropagationSpy = vi.fn()
    fireEvent.click(tmdbBadge, { stopPropagation: stopPropagationSpy })
  })
})
