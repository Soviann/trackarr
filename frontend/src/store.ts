import { create } from 'zustand'
import { apiFetch } from './api'
import { Title, PaginatedResponse, StatusCounts } from './types'

const PAGE_SIZE = 50

interface TitleState {
  titles: Title[]
  total: number
  hasMore: boolean
  counts: StatusCounts | null
  loading: boolean
  loadingMore: boolean
  error: string | null
  filter: {
    status?: string
    type?: string
    search?: string
    match_status?: string
  }
  setFilter: (filter: Partial<TitleState['filter']>) => void
  fetchTitles: () => Promise<void>
  loadMore: () => Promise<void>
  invalidate: () => Promise<void>
  getTitleById: (id: number) => Title | undefined
  updateTitleInCache: (title: Title) => void
}

export const useTitleStore = create<TitleState>((set, get) => ({
  titles: [],
  total: 0,
  hasMore: false,
  counts: null,
  loading: false,
  loadingMore: false,
  error: null,
  filter: {},

  setFilter: (filter) => {
    set({ filter: { ...get().filter, ...filter } })
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

  getTitleById: (id) => get().titles.find((t) => t.id === id),

  updateTitleInCache: (title) => {
    set({
      titles: get().titles.map((t) => (t.id === title.id ? title : t)),
    })
  },
}))
