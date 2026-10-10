import { render, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
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

vi.mock('../context/UndoContext', () => ({
  useUndo: () => ({
    showUndo: vi.fn(),
  }),
}))

const mockSearchState = {
  query: '',
  results: [],
  total: 0,
  hasMore: false,
  loading: false,
  loadingMore: false,
  error: null,
  searchOnTMDB: false,
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
})
