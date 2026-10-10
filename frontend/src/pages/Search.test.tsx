import { render, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import type { Title } from '../types'
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
})
