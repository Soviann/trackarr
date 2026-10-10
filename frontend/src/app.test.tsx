import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup, act } from '@testing-library/preact'
import { route } from 'preact-router'
import { App } from './app'
import { setLocale } from './i18n'

const mockTitleState = {
  filter: {},
  sort: { field: 'updated_at', order: 'desc' },
  setFilter: vi.fn(),
  setSort: vi.fn(),
  titles: [],
  _fetchGen: 0,
  fetchTitles: vi.fn(),
  invalidate: vi.fn(),
}

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

vi.mock('./store', () => {
  const useTitleStore = (sel?: (s: typeof mockTitleState) => unknown) =>
    sel ? sel(mockTitleState) : mockTitleState
  useTitleStore.getState = () => mockTitleState
  useTitleStore.setState = vi.fn()

  const useSearchStore = (sel?: (s: typeof mockSearchState) => unknown) =>
    sel ? sel(mockSearchState) : mockSearchState
  useSearchStore.getState = () => mockSearchState
  useSearchStore.setState = vi.fn()

  return { useTitleStore, useSearchStore }
})

vi.mock('./api', () => ({
  apiFetch: vi.fn().mockResolvedValue([]),
}))

vi.mock('./hooks/usePush', () => ({
  usePush: vi.fn(),
}))

vi.mock('./hooks/useServiceWorker', () => ({
  useServiceWorker: vi.fn(),
}))

vi.mock('./utils/badge', () => ({
  updateBadge: vi.fn(),
}))

describe('App navigation & filter drawer', () => {
  beforeEach(() => {
    setLocale('en')
    localStorage.clear()
    window.scrollTo = vi.fn()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      json: () => Promise.resolve({}),
    }))
  })

  afterEach(() => {
    cleanup()
    localStorage.clear()
    vi.unstubAllGlobals()
  })

  it('closes filter drawer when navigating to another page and returning', async () => {
    window.history.pushState({}, '', '/search')
    const { container, getByTitle } = render(<App />)

    // Open filter drawer on /search
    const filterBtn = getByTitle('Filters')
    expect(filterBtn).toBeDefined()
    expect(filterBtn.getAttribute('aria-expanded')).toBe('false')

    await act(async () => {
      fireEvent.click(filterBtn)
    })

    expect(filterBtn.getAttribute('aria-expanded')).toBe('true')
    expect(container.querySelector('.drawerExpanded')).not.toBeNull()

    // Navigate away to coming-up page
    await act(async () => {
      route('/coming-up')
    })

    // Navigate back to search page
    await act(async () => {
      route('/search')
    })

    // Drawer should now be closed
    const filterBtnAfter = getByTitle('Filters')
    expect(filterBtnAfter.getAttribute('aria-expanded')).toBe('false')
    expect(container.querySelector('.drawerExpanded')).toBeNull()
    expect(container.querySelector('.drawerCollapsed')).not.toBeNull()
  })

  it('closes filter drawer when navigating from Search directly to Library', async () => {
    window.history.pushState({}, '', '/search')
    const { container, getByTitle } = render(<App />)

    // Open filter drawer on /search
    const filterBtn = getByTitle('Filters')
    await act(async () => {
      fireEvent.click(filterBtn)
    })
    expect(container.querySelector('.drawerExpanded')).not.toBeNull()

    // Navigate directly to Library /
    await act(async () => {
      route('/')
    })

    // On Library, drawer must be closed
    expect(container.querySelector('.drawerExpanded')).toBeNull()
    expect(container.querySelector('.drawerCollapsed')).not.toBeNull()

    // Navigate back to Search
    await act(async () => {
      route('/search')
    })
    expect(container.querySelector('.drawerExpanded')).toBeNull()
    expect(container.querySelector('.drawerCollapsed')).not.toBeNull()
  })

  it('closes filter drawer when navigating away from Library and returning', async () => {
    window.history.pushState({}, '', '/')
    const { container, getByText } = render(<App />)

    // Open filter drawer on Library
    const handleText = getByText('Filters')
    await act(async () => {
      fireEvent.click(handleText)
    })
    expect(container.querySelector('.drawerExpanded')).not.toBeNull()

    // Navigate away to Stats
    await act(async () => {
      route('/stats')
    })

    // Navigate back to Library
    await act(async () => {
      route('/')
    })

    // Drawer must be closed on return
    expect(container.querySelector('.drawerExpanded')).toBeNull()
    expect(container.querySelector('.drawerCollapsed')).not.toBeNull()
  })

  it('clears search store when navigating away from /search to another page', async () => {
    window.history.pushState({}, '', '/search')
    render(<App />)
    mockSearchState.clear.mockClear()

    // Navigate away to coming-up page
    await act(async () => {
      route('/coming-up')
    })

    expect(mockSearchState.clear).toHaveBeenCalledTimes(1)
  })

  it('clears search store when exiting merge mode on /search', async () => {
    window.history.pushState({}, '', '/search?mergeSourceId=12&mergeSourceName=Frieren')
    render(<App />)
    mockSearchState.clear.mockClear()

    // Navigate to plain /search (e.g. clicking Explore tab)
    await act(async () => {
      route('/search')
    })

    expect(mockSearchState.clear).toHaveBeenCalledTimes(1)
  })

  it('does not clear search store when navigating between non-search pages', async () => {
    window.history.pushState({}, '', '/coming-up')
    render(<App />)
    mockSearchState.clear.mockClear()

    await act(async () => {
      route('/continue-watching')
    })

    expect(mockSearchState.clear).not.toHaveBeenCalled()
  })
})
