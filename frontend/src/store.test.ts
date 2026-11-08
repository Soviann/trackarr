import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useSearchStore, hasActiveFilters, type TitleFilter } from './store'
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
