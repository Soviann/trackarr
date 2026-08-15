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
})
