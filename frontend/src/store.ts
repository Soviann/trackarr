import { create } from 'zustand'
import { apiFetch } from './api'
import { Title, PaginatedResponse, StatusCounts } from './types'

const PAGE_SIZE = 48

const SORT_STORAGE_KEY = 'title-sort'

export type SortField = 'updated_at' | 'original_title' | 'release_date' | 'my_rating' | 'created_at' | 'last_watched_at'
export type SortOrder = 'asc' | 'desc'

export interface SortState {
  field: SortField
  order: SortOrder
}

const DEFAULT_SORT: SortState = { field: 'release_date', order: 'desc' }

const SORT_FIELDS: SortField[] = ['updated_at', 'original_title', 'release_date', 'my_rating', 'created_at', 'last_watched_at']

function loadSort(): SortState {
  try {
    const raw = localStorage.getItem(SORT_STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (SORT_FIELDS.includes(parsed.field) && (parsed.order === 'asc' || parsed.order === 'desc')) return parsed
    }
  } catch { /* ignore */ }
  return DEFAULT_SORT
}

function saveSort(sort: SortState) {
  localStorage.setItem(SORT_STORAGE_KEY, JSON.stringify(sort))
}

type TitleFilter = {
  status?: string
  type?: string
  is_anime?: string
  search?: string
  match_status?: string
  series_status?: string
  decade?: string
  release_from?: string
  release_to?: string
  include_no_release?: string
  genres?: string[]
  genre_op?: 'AND' | 'OR'
}

function buildFilterParams(filter: TitleFilter, sort?: SortState): URLSearchParams {
  const params = new URLSearchParams()
  if (filter.status) params.set('status', filter.status)
  if (filter.type) params.set('type', filter.type)
  if (filter.is_anime) params.set('is_anime', filter.is_anime)
  if (filter.search) params.set('search', filter.search)
  if (filter.match_status) params.set('match_status', filter.match_status)
  if (filter.series_status) params.set('series_status', filter.series_status)
  if (filter.decade) params.set('decade', filter.decade)
  if (filter.release_from) params.set('release_from', filter.release_from)
  if (filter.release_to) params.set('release_to', filter.release_to)
  if (filter.include_no_release) params.set('include_no_release', filter.include_no_release)
  if (filter.genres && filter.genres.length > 0) {
    filter.genres.forEach(g => params.append('genres', g))
    if (filter.genre_op) params.set('genre_op', filter.genre_op)
  }
  if (sort) {
    params.set('sort', sort.field)
    params.set('order', sort.order)
  }
  return params
}

interface TitleState {
  titles: Title[]
  total: number
  hasMore: boolean
  counts: StatusCounts | null
  loading: boolean
  loadingMore: boolean
  error: string | null
  sort: SortState
  filter: TitleFilter
  _fetchGen: number
  setFilter: (filter: Partial<TitleFilter>) => void
  setSort: (sort: SortState) => void
  fetchTitles: () => Promise<void>
  loadMore: () => Promise<void>
  invalidate: () => Promise<void>
}

export const useTitleStore = create<TitleState>((set, get) => ({
  titles: [],
  total: 0,
  hasMore: false,
  counts: null,
  loading: false,
  loadingMore: false,
  error: null,
  sort: loadSort(),
  filter: {},
  _fetchGen: 0,

  setFilter: (filter) => {
    set({ filter: { ...get().filter, ...filter }, titles: [], _fetchGen: get()._fetchGen + 1 })
    get().fetchTitles()
  },

  setSort: (sort) => {
    set({ sort, titles: [], _fetchGen: get()._fetchGen + 1 })
    saveSort(sort)
    get().fetchTitles()
  },

  fetchTitles: async () => {
    const { titles } = get()
    const isFirstLoad = titles.length === 0
    const gen = get()._fetchGen + 1
    if (isFirstLoad) {
      set({ loading: true, error: null, _fetchGen: gen })
    } else {
      set({ error: null, _fetchGen: gen })
    }
    try {
      const f = get().filter
      const params = buildFilterParams(f, f.search ? undefined : get().sort)
      const limit = isFirstLoad ? PAGE_SIZE : Math.max(titles.length, PAGE_SIZE)
      params.set('limit', String(limit))
      params.set('offset', '0')
      const qs = params.toString()
      const result = await apiFetch<PaginatedResponse>(`/titles?${qs}`)
      if (get()._fetchGen !== gen) return
      set({
        titles: result.titles,
        total: result.total,
        hasMore: result.has_more,
        counts: result.counts ?? get().counts,
        loading: false,
      })
    } catch (e) {
      if (get()._fetchGen !== gen) return
      set({ error: e instanceof Error ? e.message : 'Fetch failed', loading: false })
    }
  },

  loadMore: async () => {
    const { titles, hasMore, loadingMore, filter, _fetchGen } = get()
    if (!hasMore || loadingMore) return
    const offset = titles.length
    const gen = _fetchGen
    set({ loadingMore: true })
    try {
      const params = buildFilterParams(filter, filter.search ? undefined : get().sort)
      params.set('limit', String(PAGE_SIZE))
      params.set('offset', String(offset))
      const qs = params.toString()
      const result = await apiFetch<PaginatedResponse>(`/titles?${qs}`)
      if (get()._fetchGen !== gen) {
        set({ loadingMore: false })
        return
      }
      set((state) => ({
        titles: [...state.titles, ...result.titles],
        total: result.total,
        hasMore: result.has_more,
        loadingMore: false,
      }))
    } catch (e) {
      if (get()._fetchGen !== gen) {
        set({ loadingMore: false })
        return
      }
      set({ error: e instanceof Error ? e.message : 'Load more failed', loadingMore: false })
    }
  },

  invalidate: async () => {
    await get().fetchTitles()
  },
}))

export interface SearchState {
  query: string
  results: Title[]
  total: number
  hasMore: boolean
  loading: boolean
  loadingMore: boolean
  error: string | null
  _searchGen: number
  setQuery: (q: string) => void
  search: (filter: TitleState['filter']) => Promise<void>
  loadMore: (filter: TitleState['filter']) => Promise<void>
  clear: () => void
}

export const useSearchStore = create<SearchState>((set, get) => ({
  query: '',
  results: [],
  total: 0,
  hasMore: false,
  loading: false,
  loadingMore: false,
  error: null,
  _searchGen: 0,

  setQuery: (query) => set({ query }),

  search: async (filter) => {
    const { query, results } = get()
    const trimmed = query.trim()
    if (!trimmed) {
      set({ results: [], total: 0, hasMore: false, error: null, loading: false })
      return
    }

    const isFirstLoad = results.length === 0
    const gen = get()._searchGen + 1
    if (isFirstLoad) {
      set({ loading: true, error: null, _searchGen: gen })
    } else {
      set({ error: null, _searchGen: gen })
    }

    try {
      const params = new URLSearchParams()
      params.set('search', trimmed)
      if (filter.status) params.set('status', filter.status)
      if (filter.type) params.set('type', filter.type)
      if (filter.is_anime) params.set('is_anime', filter.is_anime)
      if (filter.series_status) params.set('series_status', filter.series_status)
      if (filter.decade) params.set('decade', filter.decade)
      if (filter.release_from) params.set('release_from', filter.release_from)
      if (filter.release_to) params.set('release_to', filter.release_to)
      if (filter.include_no_release) params.set('include_no_release', filter.include_no_release)
      if (filter.genres && filter.genres.length > 0) {
        filter.genres.forEach(g => params.append('genres', g))
        if (filter.genre_op) params.set('genre_op', filter.genre_op)
      }

      const limit = isFirstLoad ? PAGE_SIZE : Math.max(results.length, PAGE_SIZE)
      params.set('limit', String(limit))
      params.set('offset', '0')

      const result = await apiFetch<PaginatedResponse>(`/titles?${params.toString()}`)
      // Discard stale response if a newer search was fired while awaiting
      if (get()._searchGen !== gen) return
      set({
        results: result.titles,
        total: result.total,
        hasMore: result.has_more,
        loading: false,
      })
    } catch (e) {
      if (get()._searchGen !== gen) return
      set({ error: e instanceof Error ? e.message : 'Search failed', loading: false })
    }
  },

  loadMore: async (filter) => {
    const { query, results, hasMore, loadingMore, _searchGen } = get()
    const trimmed = query.trim()
    if (!trimmed || !hasMore || loadingMore) return

    const offset = results.length
    const gen = _searchGen
    set({ loadingMore: true })
    try {
      const params = new URLSearchParams()
      params.set('search', trimmed)
      if (filter.status) params.set('status', filter.status)
      if (filter.type) params.set('type', filter.type)
      if (filter.is_anime) params.set('is_anime', filter.is_anime)
      if (filter.series_status) params.set('series_status', filter.series_status)
      if (filter.decade) params.set('decade', filter.decade)
      if (filter.release_from) params.set('release_from', filter.release_from)
      if (filter.release_to) params.set('release_to', filter.release_to)
      if (filter.include_no_release) params.set('include_no_release', filter.include_no_release)
      if (filter.genres && filter.genres.length > 0) {
        filter.genres.forEach(g => params.append('genres', g))
        if (filter.genre_op) params.set('genre_op', filter.genre_op)
      }

      params.set('limit', String(PAGE_SIZE))
      params.set('offset', String(offset))

      const result = await apiFetch<PaginatedResponse>(`/titles?${params.toString()}`)
      if (get()._searchGen !== gen) {
        set({ loadingMore: false })
        return
      }
      set((state) => ({
        results: [...state.results, ...result.titles],
        total: result.total,
        hasMore: result.has_more,
        loadingMore: false,
      }))
    } catch (e) {
      if (get()._searchGen !== gen) {
        set({ loadingMore: false })
        return
      }
      set({ error: e instanceof Error ? e.message : 'Load more failed', loadingMore: false })
    }
  },

  clear: () => set({ query: '', results: [], total: 0, hasMore: false, error: null }),
}))
