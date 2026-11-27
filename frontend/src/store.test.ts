import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useSearchStore, hasActiveFilters, type TitleFilter } from './store'
import { routeTo } from './routes'
import { apiFetch } from './api'

vi.mock('./api', () => ({
  apiFetch: vi.fn(),
}))

describe('hasActiveFilters', () => {
  it('returns false for undefined or empty filter', () => {
    expect(hasActiveFilters(undefined)).toBe(false)
    expect(hasActiveFilters({})).toBe(false)
  })

  it('returns true when status is set', () => {
    expect(hasActiveFilters({ status: 'watching' })).toBe(true)
  })

  it('returns true when type is set', () => {
    expect(hasActiveFilters({ type: 'movie' })).toBe(true)
  })

  it('returns true when is_anime is set', () => {
    expect(hasActiveFilters({ is_anime: 'true' })).toBe(true)
  })

  it('returns true when genres array has items', () => {
    expect(hasActiveFilters({ genres: [] })).toBe(false)
    expect(hasActiveFilters({ genres: ['Action'] })).toBe(true)
  })

  it('returns true when origin_country array has items', () => {
    expect(hasActiveFilters({ origin_country: [] })).toBe(false)
    expect(hasActiveFilters({ origin_country: ['JP'] })).toBe(true)
  })

  it('returns true when decade or release date range is set', () => {
    expect(hasActiveFilters({ decade: '1990' })).toBe(true)
    expect(hasActiveFilters({ release_from: '2020-01-01' })).toBe(true)
    expect(hasActiveFilters({ release_to: '2025-12-31' })).toBe(true)
  })

  it('returns true when ratings are set', () => {
    expect(hasActiveFilters({ my_rating_min: '8' })).toBe(true)
    expect(hasActiveFilters({ tmdb_rating_min: '7' })).toBe(true)
  })

  it('returns true when match_status or series_status is set', () => {
    expect(hasActiveFilters({ match_status: 'waiting' })).toBe(true)
    expect(hasActiveFilters({ series_status: 'continuing' })).toBe(true)
  })
})

describe('useSearchStore - filter search without text query', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useSearchStore.getState().clear()
  })

  it('does not call apiFetch and clears results when query is empty and no filters are active', async () => {
    await useSearchStore.getState().search()

    expect(apiFetch).not.toHaveBeenCalled()
    expect(useSearchStore.getState().results).toEqual([])
    expect(useSearchStore.getState().total).toBe(0)
    expect(useSearchStore.getState().loading).toBe(false)
  })

  it('calls /titles with filter params when query is empty but filter is active', async () => {
    const mockTitles = [{ id: 1, title: 'Test Anime' }]
    vi.mocked(apiFetch).mockResolvedValueOnce({
      titles: mockTitles,
      total: 1,
      has_more: false,
    })

    const filter: TitleFilter = { is_anime: 'true', genres: ['Action'] }
    await useSearchStore.getState().search(filter)

    expect(apiFetch).toHaveBeenCalledTimes(1)
    const calledUrl = vi.mocked(apiFetch).mock.calls[0][0] as string
    expect(calledUrl).toContain('/titles?')
    expect(calledUrl).toContain('is_anime=true')
    expect(calledUrl).toContain('genres=Action')
    expect(calledUrl).not.toContain('search=')

    expect(useSearchStore.getState().results).toEqual(mockTitles)
    expect(useSearchStore.getState().total).toBe(1)
  })

  it('allows loadMore when query is empty but filter is active', async () => {
    useSearchStore.setState({
      query: '',
      results: [{ id: 1 } as any],
      total: 2,
      hasMore: true,
      loadingMore: false,
    })

    vi.mocked(apiFetch).mockResolvedValueOnce({
      titles: [{ id: 2 }],
      total: 2,
      has_more: false,
    })

    const filter: TitleFilter = { type: 'movie' }
    await useSearchStore.getState().loadMore(filter)

    expect(apiFetch).toHaveBeenCalledTimes(1)
    const calledUrl = vi.mocked(apiFetch).mock.calls[0][0] as string
    expect(calledUrl).toContain('type=movie')
    expect(calledUrl).toContain('offset=1')

    expect(useSearchStore.getState().results).toHaveLength(2)
    expect(useSearchStore.getState().hasMore).toBe(false)
  })
})

describe('routeTo.person', () => {
  it('builds /person/:name without query when role is omitted or all', () => {
    expect(routeTo.person('Denis Villeneuve')).toBe('/person/Denis%20Villeneuve')
    expect(routeTo.person('Denis Villeneuve', 'all')).toBe('/person/Denis%20Villeneuve')
  })

  it('appends ?role= when a specific role is provided', () => {
    expect(routeTo.person('Hans Zimmer', 'composer')).toBe('/person/Hans%20Zimmer?role=composer')
    expect(routeTo.person('Christopher Nolan', 'writer')).toBe('/person/Christopher%20Nolan?role=writer')
  })
})

describe('useSearchStore - people search', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useSearchStore.getState().clear()
  })

  it('fetches /titles and /people concurrently for non-empty query', async () => {
    const mockPeople = [{ name: 'Hans Zimmer', roles: ['composer'], title_count: 5 }]
    vi.mocked(apiFetch).mockImplementation((url: string) => {
      if (url.startsWith('/people?')) {
        return Promise.resolve(mockPeople)
      }
      return Promise.resolve({ titles: [], total: 0, has_more: false })
    })

    useSearchStore.getState().setQuery('Hans Zimmer')
    await useSearchStore.getState().search()

    expect(apiFetch).toHaveBeenCalledTimes(2)
    expect(apiFetch).toHaveBeenCalledWith('/people?search=Hans%20Zimmer')
    expect(useSearchStore.getState().peopleResults).toEqual(mockPeople)
    expect(useSearchStore.getState().loadingPeople).toBe(false)
    expect(useSearchStore.getState().loading).toBe(false)
  })

  it('clears peopleResults and skips /people when query is blank even with active filters', async () => {
    useSearchStore.setState({
      query: '   ',
      peopleResults: [{ name: 'Stale Person', roles: ['actor'], title_count: 1 }],
    })
    vi.mocked(apiFetch).mockResolvedValueOnce({ titles: [], total: 0, has_more: false })

    await useSearchStore.getState().search({ type: 'movie' })

    expect(apiFetch).toHaveBeenCalledTimes(1)
    expect(vi.mocked(apiFetch).mock.calls[0][0]).toContain('/titles?')
    expect(useSearchStore.getState().peopleResults).toEqual([])
    expect(useSearchStore.getState().loadingPeople).toBe(false)
  })

  it('suppresses /people fetch and clears peopleResults when includePeople is false', async () => {
    useSearchStore.setState({
      query: 'Dune',
      peopleResults: [{ name: 'Denis Villeneuve', roles: ['director'], title_count: 2 }],
    })
    vi.mocked(apiFetch).mockResolvedValueOnce({ titles: [], total: 0, has_more: false })

    await useSearchStore.getState().search({}, { includePeople: false })

    expect(apiFetch).toHaveBeenCalledTimes(1)
    expect(vi.mocked(apiFetch).mock.calls[0][0]).toContain('/titles?')
    expect(useSearchStore.getState().peopleResults).toEqual([])
    expect(useSearchStore.getState().loadingPeople).toBe(false)
  })

  it('discards stale /people and /titles responses via _searchGen race guard', async () => {
    let resolveFirstPeople!: (value: unknown) => void
    let resolveFirstTitles!: (value: unknown) => void
    const firstPeople = new Promise((r) => { resolveFirstPeople = r })
    const firstTitles = new Promise((r) => { resolveFirstTitles = r })

    vi.mocked(apiFetch)
      .mockImplementationOnce(() => firstTitles)
      .mockImplementationOnce(() => firstPeople)
      .mockImplementationOnce(() => Promise.resolve({ titles: [{ id: 2 }], total: 1, has_more: false }))
      .mockImplementationOnce(() => Promise.resolve([{ name: 'Second Person', roles: ['writer'], title_count: 3 }]))

    useSearchStore.getState().setQuery('First')
    const p1 = useSearchStore.getState().search()

    useSearchStore.getState().setQuery('Second')
    const p2 = useSearchStore.getState().search()
    await p2

    resolveFirstTitles({ titles: [{ id: 1 }], total: 1, has_more: false })
    resolveFirstPeople([{ name: 'First Person', roles: ['actor'], title_count: 1 }])
    await p1

    expect(useSearchStore.getState().results).toEqual([{ id: 2 }])
    expect(useSearchStore.getState().peopleResults).toEqual([
      { name: 'Second Person', roles: ['writer'], title_count: 3 },
    ])
  })

  it('degrades gracefully when /people fails while /titles succeeds', async () => {
    useSearchStore.setState({
      query: 'Nolan',
      peopleResults: [{ name: 'Old Person', roles: ['director'], title_count: 1 }],
    })
    vi.mocked(apiFetch).mockImplementation((url: string) => {
      if (url.startsWith('/people?')) {
        return Promise.reject(new Error('network down'))
      }
      return Promise.resolve({ titles: [{ id: 10 }], total: 1, has_more: false })
    })

    await useSearchStore.getState().search()

    expect(useSearchStore.getState().results).toEqual([{ id: 10 }])
    expect(useSearchStore.getState().peopleResults).toEqual([])
    expect(useSearchStore.getState().loadingPeople).toBe(false)
    expect(useSearchStore.getState().error).toBeNull()
  })

  it('clears peopleResults and discards in-flight results on clear()', async () => {
    let resolvePeople!: (value: unknown) => void
    const pendingPeople = new Promise((r) => { resolvePeople = r })

    vi.mocked(apiFetch).mockImplementation((url: string) => {
      if (url.startsWith('/people?')) return pendingPeople
      return Promise.resolve({ titles: [], total: 0, has_more: false })
    })

    useSearchStore.getState().setQuery('Hans')
    const searchPromise = useSearchStore.getState().search()
    useSearchStore.getState().clear()

    resolvePeople([{ name: 'Hans Zimmer', roles: ['composer'], title_count: 2 }])
    await searchPromise

    expect(useSearchStore.getState().query).toBe('')
    expect(useSearchStore.getState().peopleResults).toEqual([])
    expect(useSearchStore.getState().loadingPeople).toBe(false)
  })
})
