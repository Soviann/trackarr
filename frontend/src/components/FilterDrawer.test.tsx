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
    expect(getByText('Status: Watching')).toBeDefined()
    expect(queryByText('TMDB: any')).toBeNull()

    // Switch to Dates & Ratings tab
    fireEvent.click(getByText('Dates & Ratings'))
    expect(getByText('TMDB: any')).toBeDefined()
    expect(queryByText('Status: Watching')).toBeNull()

    // Switch to Genres & Origin tab
    fireEvent.click(getByText('Genres & Origin'))
    expect(queryByText('TMDB: any')).toBeNull()
    expect(queryByText('Status: Watching')).toBeNull()
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

  it('accepts consolidated filter and actions props and handles changes', () => {
    const onStatusChange = vi.fn()
    const onTypeChange = vi.fn()
    const onSortChange = vi.fn()
    const { getByLabelText, getByText } = render(
      <FilterDrawer
        defaultOpen={true}
        sort={{ field: 'updated_at', order: 'desc' }}
        onSortChange={onSortChange}
        isSearchActive={false}
        filter={{
          status: 'plan_to_watch',
          type: 'movie',
          isAnime: false,
          seriesStatus: null,
          decade: null,
          releaseFrom: '',
          releaseTo: '',
          includeNoRelease: true,
          selectedGenres: [],
          genreOp: 'OR',
          selectedCountries: [],
          myRatingMin: '',
          tmdbRatingMin: '',
        }}
        actions={{
          onStatusChange,
          onTypeChange,
          onIsAnimeChange: vi.fn(),
          onSeriesStatusChange: vi.fn(),
          onDecadeChange: vi.fn(),
          onReleaseFromChange: vi.fn(),
          onReleaseToChange: vi.fn(),
          onIncludeNoReleaseChange: vi.fn(),
          onGenreToggle: vi.fn(),
          onGenreOpChange: vi.fn(),
          onCountryToggle: vi.fn(),
          onMyRatingMinChange: vi.fn(),
          onTmdbRatingMinChange: vi.fn(),
        }}
      />
    )
    const statusSelect = getByLabelText('Filter status') as HTMLSelectElement
    fireEvent.change(statusSelect, { target: { value: 'watching' } })
    expect(onStatusChange).toHaveBeenCalledWith('watching')

    const seriesBtn = getByText('Series')
    fireEvent.click(seriesBtn)
    expect(onTypeChange).toHaveBeenCalledWith('series')

    const sortSelect = getByLabelText('Sort by') as HTMLSelectElement
    fireEvent.change(sortSelect, { target: { value: 'original_title' } })
    expect(onSortChange).toHaveBeenCalledWith({ field: 'original_title', order: 'asc' })

    const orderBtn = getByLabelText('Descending')
    fireEvent.click(orderBtn)
    expect(onSortChange).toHaveBeenCalledWith({ field: 'updated_at', order: 'asc' })
  })

  it('renders single-line closed handle with active count and dismissible chips', () => {
    const onStatusChange = vi.fn()
    const onIsAnimeChange = vi.fn()
    const onGenreToggle = vi.fn()
    const { container, getByText, getByLabelText } = renderFilterDrawer({
      defaultOpen: false,
      sort: { field: 'release_date', order: 'desc' },
      status: 'watching',
      isAnime: true,
      selectedGenres: ['Action'],
      onStatusChange,
      onIsAnimeChange,
      onGenreToggle,
    })

    // Active count in handle button
    expect(getByText('(3)')).toBeDefined()

    // Dismissible chips are visible when closed
    expect(getByText('Watching')).toBeDefined()
    expect(getByText('Anime')).toBeDefined()
    expect(getByText('Action')).toBeDefined()

    // Dismiss Watching chip
    const removeWatchingBtn = getByLabelText('Remove filter Watching')
    fireEvent.click(removeWatchingBtn)
    expect(onStatusChange).toHaveBeenCalledWith(null)

    // Verify click on dismiss does not expand drawer
    const drawerEl = container.querySelector('.drawer')
    expect(drawerEl?.className).toContain('drawerCollapsed')

    // Dismiss Anime chip
    const removeAnimeBtn = getByLabelText('Remove filter Anime')
    fireEvent.click(removeAnimeBtn)
    expect(onIsAnimeChange).toHaveBeenCalledWith(false)

    // Dismiss Action genre chip
    const removeActionBtn = getByLabelText('Remove filter Action')
    fireEvent.click(removeActionBtn)
    expect(onGenreToggle).toHaveBeenCalledWith('Action')
  })
})

