import { create } from 'zustand'
import { apiFetch } from './api'
import { Title } from './types'

interface TitleState {
  titles: Title[]
  loading: boolean
  error: string | null
  filter: {
    status?: string
    type?: string
    search?: string
    match_status?: string
  }
  setFilter: (filter: Partial<TitleState['filter']>) => void
  fetchTitles: () => Promise<void>
  invalidate: () => Promise<void>
  getTitleById: (id: number) => Title | undefined
  updateTitleInCache: (title: Title) => void
}

export const useTitleStore = create<TitleState>((set, get) => ({
  titles: [],
  loading: false,
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
      const qs = params.toString()
      const titles = await apiFetch<Title[]>(`/titles${qs ? '?' + qs : ''}`)
      set({ titles, loading: false })
    } catch (e) {
      set({ error: e instanceof Error ? e.message : 'Fetch failed', loading: false })
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
