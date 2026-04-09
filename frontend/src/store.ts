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

function loadSort(): SortState {
  try {
    const raw = localStorage.getItem(SORT_STORAGE_KEY)
    if (raw) {
      const parsed = JSON.parse(raw)
      if (parsed.field && parsed.order) return parsed
    }
  } catch { /* ignore */ }
  return DEFAULT_SORT
}

function saveSort(sort: SortState) {
  localStorage.setItem(SORT_STORAGE_KEY, JSON.stringify(sort))
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
  filter: {
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
  }
  setFilter: (filter: Partial<TitleState['filter']>) => void
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

  setFilter: (filter) => {
    set({ filter: { ...get().filter, ...filter }, titles: [] })
    get().fetchTitles()
  },

  setSort: (sort) => {
    set({ sort, titles: [] })
    saveSort(sort)
    get().fetchTitles()
  },

  fetchTitles: async () => {
    const { titles } = get()
    const isFirstLoad = titles.length === 0
    if (isFirstLoad) {
      set({ loading: true, error: null })
    } else {
      set({ error: null })
    }
    try {
      const params = new URLSearchParams()
      const f = get().filter
      if (f.status) params.set('status', f.status)
      if (f.type) params.set('type', f.type)
      if (f.is_anime) params.set('is_anime', f.is_anime)
      if (f.search) params.set('search', f.search)
      if (f.match_status) params.set('match_status', f.match_status)
      if (f.series_status) params.set('series_status', f.series_status)
      if (f.decade) params.set('decade', f.decade)
      if (f.release_from) params.set('release_from', f.release_from)
      if (f.release_to) params.set('release_to', f.release_to)
      if (f.include_no_release) params.set('include_no_release', f.include_no_release)
      if (!f.search) {
        params.set('sort', get().sort.field)
        params.set('order', get().sort.order)
      }
      const limit = isFirstLoad ? PAGE_SIZE : Math.max(titles.length, PAGE_SIZE)
      params.set('limit', String(limit))
      params.set('offset', '0')
      const qs = params.toString()
      const result = await apiFetch<PaginatedResponse>(`/titles?${qs}`)
      set({
        titles: result.titles,
        total: result.total,
        hasMore: result.has_more,
        counts: result.counts ?? get().counts,
        loading: false,
      })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'Fetch failed', loading: false })
    }
  },

  loadMore: async () => {
    const { titles, hasMore, loadingMore, filter } = get()
    if (!hasMore || loadingMore) return
    set({ loadingMore: true })
    try {
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
      if (!filter.search) {
        params.set('sort', get().sort.field)
        params.set('order', get().sort.order)
      }
      params.set('limit', String(PAGE_SIZE))
      params.set('offset', String(titles.length))
      const qs = params.toString()
      const result = await apiFetch<PaginatedResponse>(`/titles?${qs}`)
      set({
        titles: [...titles, ...result.titles],
        total: result.total,
        hasMore: result.has_more,
        loadingMore: false,
      })
    } catch (e) {
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

  setQuery: (query) => set({ query }),

  search: async (filter) => {
    const { query, results } = get()
    const trimmed = query.trim()
    if (!trimmed) {
      set({ results: [], total: 0, hasMore: false, error: null, loading: false })
      return
    }

    const isFirstLoad = results.length === 0
    if (isFirstLoad) {
      set({ loading: true, error: null })
    } else {
      set({ error: null })
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
      
      const limit = isFirstLoad ? PAGE_SIZE : Math.max(results.length, PAGE_SIZE)
      params.set('limit', String(limit))
      params.set('offset', '0')
      
      const result = await apiFetch<PaginatedResponse>(`/titles?${params.toString()}`)
      set({
        results: result.titles,
        total: result.total,
        hasMore: result.has_more,
        loading: false,
      })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'Search failed', loading: false })
    }
  },

  loadMore: async (filter) => {
    const { query, results, hasMore, loadingMore } = get()
    const trimmed = query.trim()
    if (!trimmed || !hasMore || loadingMore) return

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
      
      params.set('limit', String(PAGE_SIZE))
      params.set('offset', String(results.length))
      
      const result = await apiFetch<PaginatedResponse>(`/titles?${params.toString()}`)
      set({
        results: [...results, ...result.titles],
        total: result.total,
        hasMore: result.has_more,
        loadingMore: false,
      })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'Load more failed', loadingMore: false })
    }
  },

  clear: () => set({ query: '', results: [], total: 0, hasMore: false, error: null }),
}))
