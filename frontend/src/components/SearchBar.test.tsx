import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { SearchBar } from './SearchBar'

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
})

