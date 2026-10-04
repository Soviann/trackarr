import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { route } from 'preact-router'
import { SearchBar } from './SearchBar'

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

const mockState = {
  query: '',
  setQuery: vi.fn((q: string) => { mockState.query = q }),
  clear: vi.fn(() => { mockState.query = '' }),
  searchOnTMDB: false,
  setSearchOnTMDB: vi.fn((v: boolean) => { mockState.searchOnTMDB = v }),
}

vi.mock('../store', () => ({
  useSearchStore: (selector: (s: typeof mockState) => unknown) => selector(mockState),
}))

describe('SearchBar', () => {
  beforeEach(() => {
    mockState.query = ''
    mockState.searchOnTMDB = false
    vi.clearAllMocks()
  })

  afterEach(() => {
    cleanup()
  })

  it('renders input with translated placeholder', () => {
    const { getByPlaceholderText } = render(<SearchBar />)
    expect(getByPlaceholderText('Search titles...')).toBeDefined()
  })

  it('updates store query on user input', () => {
    const { getByPlaceholderText } = render(<SearchBar />)
    const input = getByPlaceholderText('Search titles...') as HTMLInputElement
    fireEvent.input(input, { target: { value: 'Arcane' } })
    expect(mockState.setQuery).toHaveBeenCalledWith('Arcane')
  })

  it('shows clear button when query is present and clears query on click', () => {
    mockState.query = 'Arcane'
    const { getByTitle } = render(<SearchBar />)

    const clearBtn = getByTitle('Clear search text')
    expect(clearBtn).toBeDefined()

    fireEvent.click(clearBtn)
    expect(mockState.clear).toHaveBeenCalledTimes(1)
  })

  it('toggles TMDB search state when TMDB button is clicked', () => {
    const { getByTitle } = render(<SearchBar showTMDBToggle={true} />)
    const tmdbBtn = getByTitle('Also search TMDB')

    fireEvent.click(tmdbBtn)
    expect(mockState.setSearchOnTMDB).toHaveBeenCalledWith(true)
  })

  it('renders filter trigger button and calls onToggleFilters on click', () => {
    const onToggleFilters = vi.fn()
    const { getByTitle } = render(
      <SearchBar
        onToggleFilters={onToggleFilters}
        isFiltersOpen={false}
        activeFilterCount={0}
      />
    )

    const filterBtn = getByTitle('Filters')
    expect(filterBtn).toBeDefined()

    fireEvent.click(filterBtn)
    expect(onToggleFilters).toHaveBeenCalledTimes(1)
  })

  it('displays active filter count badge when activeFilterCount > 0', () => {
    const { getByText, container } = render(
      <SearchBar
        onToggleFilters={vi.fn()}
        isFiltersOpen={true}
        activeFilterCount={3}
      />
    )

    expect(getByText('3')).toBeDefined()
    const badge = container.querySelector('.filterCountBadge')
    expect(badge).not.toBeNull()
    expect(badge?.textContent).toBe('3')
  })

  it('redirects to /admin/validate when a supported media URL is entered', () => {
    const { getByPlaceholderText } = render(<SearchBar />)
    const input = getByPlaceholderText('Search titles...') as HTMLInputElement
    fireEvent.input(input, { target: { value: 'https://www.themoviedb.org/movie/550-fight-club' } })
    expect(route).toHaveBeenCalledWith('/admin/validate?q=https%3A%2F%2Fwww.themoviedb.org%2Fmovie%2F550-fight-club')
    expect(mockState.setQuery).not.toHaveBeenCalled()
  })

  it('redirects to /admin/validate when an IMDb URL is entered', () => {
    const { getByPlaceholderText } = render(<SearchBar />)
    const input = getByPlaceholderText('Search titles...') as HTMLInputElement
    fireEvent.input(input, { target: { value: 'https://www.imdb.com/title/tt0137523/' } })
    expect(route).toHaveBeenCalledWith('/admin/validate?q=https%3A%2F%2Fwww.imdb.com%2Ftitle%2Ftt0137523%2F')
    expect(mockState.setQuery).not.toHaveBeenCalled()
  })
})

