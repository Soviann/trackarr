import { create } from 'zustand'
import { apiFetch } from './api'
import { Title, PaginatedResponse, StatusCounts } from './types'

const PAGE_SIZE = 48

const SORT_STORAGE_KEY = 'title-sort'

export type SortField = 'updated_at' | 'original_title' | 'year' | 'my_rating' | 'created_at'
export type SortOrder = 'asc' | 'desc'

export interface SortState {
  field: SortField
  order: SortOrder
}

const DEFAULT_SORT: SortState = { field: 'updated_at', order: 'desc' }

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
    search?: string
    match_status?: string
    series_status?: string
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
  filter: { status: 'plan_to_watch' },

  setFilter: (filter) => {
    set({ filter: { ...get().filter, ...filter } })
    get().fetchTitles()
  },

  setSort: (sort) => {
    set({ sort })
    saveSort(sort)
    get().fetchTitles()
  },

  fetchTitles: async () => {
    set({ loading: true, error: null })
    try {
      const params = new URLSearchParams()
      const f = get().filter
      if (f.status) params.set('status', f.status)
      if (f.type) params.set('type', f.type)
      if (f.search) params.set('search', f.search)
      if (f.match_status) params.set('match_status', f.match_status)
      if (f.series_status) params.set('series_status', f.series_status)
      if (!f.search) {
        params.set('sort', get().sort.field)
        params.set('order', get().sort.order)
      }
      params.set('limit', String(PAGE_SIZE))
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
      if (filter.search) params.set('search', filter.search)
      if (filter.match_status) params.set('match_status', filter.match_status)
      if (filter.series_status) params.set('series_status', filter.series_status)
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
