import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { FilterDrawer } from './FilterDrawer'

const STORAGE_KEY = 'filter-drawer-open-home-v2'

function renderFilterDrawer(overrides: Partial<Parameters<typeof FilterDrawer>[0]> = {}) {
  const defaultProps: Parameters<typeof FilterDrawer>[0] = {
    status: null,
    type: null,
    isAnime: false,
    seriesStatus: null,
    onStatusChange: vi.fn(),
    onTypeChange: vi.fn(),
    onIsAnimeChange: vi.fn(),
    onSeriesStatusChange: vi.fn(),
    sort: { field: 'updated_at', order: 'desc' },
    onSortChange: vi.fn(),
    isSearchActive: false,
    defaultOpen: false,
    decade: null,
    releaseFrom: '',
    releaseTo: '',
    includeNoRelease: true,
    onDecadeChange: vi.fn(),
    onReleaseFromChange: vi.fn(),
    onReleaseToChange: vi.fn(),
    onIncludeNoReleaseChange: vi.fn(),
    selectedGenres: [],
    genreOp: 'OR',
    onGenreToggle: vi.fn(),
    onGenreOpChange: vi.fn(),
    selectedCountries: [],
    onCountryToggle: vi.fn(),
    myRatingMin: '',
    tmdbRatingMin: '',
    onMyRatingMinChange: vi.fn(),
    onTmdbRatingMinChange: vi.fn(),
    ...overrides,
  }
  return render(<FilterDrawer {...defaultProps} />)
}

describe('FilterDrawer', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    cleanup()
    localStorage.clear()
  })

  it('renders closed by default when no localStorage entry exists', () => {
    const { container } = renderFilterDrawer()
    const drawerEl = container.querySelector('.drawer')
    expect(drawerEl).not.toBeNull()
    expect(drawerEl?.className).toContain('drawerCollapsed')
    expect(drawerEl?.className).not.toContain('drawerExpanded')
  })

  it('expands when handle is clicked and persists choice to localStorage', () => {
    const { container, getByText } = renderFilterDrawer()
    const handleText = getByText('Filters')
    
    // Click to open
    fireEvent.click(handleText)
    const drawerEl = container.querySelector('.drawer')
    expect(drawerEl?.className).toContain('drawerExpanded')
    expect(localStorage.getItem(STORAGE_KEY)).toBe('true')
  })

  it('restores open state from localStorage when previously saved as true', () => {
    localStorage.setItem(STORAGE_KEY, 'true')
    const { container } = renderFilterDrawer()
    const drawerEl = container.querySelector('.drawer')
    expect(drawerEl?.className).toContain('drawerExpanded')
  })

  it('renders 3 tabs and switches between them when clicked', () => {
    const { getByText, queryByText } = renderFilterDrawer({ defaultOpen: true })
    
    // Default tab is Status & Type
    expect(getByText('Status & Type')).toBeDefined()
    expect(getByText('Genres & Origin')).toBeDefined()
    expect(getByText('Dates & Ratings')).toBeDefined()
    expect(getByText('Watching')).toBeDefined()
    expect(queryByText('TMDB: any')).toBeNull()

    // Switch to Dates & Ratings tab
    fireEvent.click(getByText('Dates & Ratings'))
    expect(getByText('TMDB: any')).toBeDefined()
    expect(queryByText('Watching')).toBeNull()

    // Switch to Genres & Origin tab
    fireEvent.click(getByText('Genres & Origin'))
    expect(queryByText('TMDB: any')).toBeNull()
    expect(queryByText('Watching')).toBeNull()
  })

  it('displays tab dot indicator when a tab has active filters', () => {
    const { container } = renderFilterDrawer({
      defaultOpen: true,
      status: 'watching',
      myRatingMin: '8',
    })

    // Check that tabDot elements are rendered
    const dots = container.querySelectorAll('.tabDot')
    expect(dots.length).toBe(2) // basics and dates
  })

  it('renders header with active count and triggers onReset when clicked', () => {
    const onReset = vi.fn()
    const { getByText, getByTitle } = renderFilterDrawer({
      defaultOpen: true,
      status: 'watching',
      type: 'series',
      sort: { field: 'release_date', order: 'desc' },
      onReset,
    })

    // Header should indicate active count
    expect(getByText('FILTERS (2 ACTIVE)')).toBeDefined()

    // Reset button should trigger onReset callback
    const resetBtn = getByTitle('Reset')
    fireEvent.click(resetBtn)
    expect(onReset).toHaveBeenCalledTimes(1)
  })
})

