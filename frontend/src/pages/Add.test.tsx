import { render, screen, waitFor, fireEvent, cleanup } from '@testing-library/preact'
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest'
import { Add } from './Add'
import { apiFetch } from '../api'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('preact-router', () => ({
  route: vi.fn(),
}))

const mockSearchStoreState = {
  query: '',
  setQuery: vi.fn(),
  setSearchOnTMDB: vi.fn(),
}

vi.mock('../store', () => ({
  useSearchStore: Object.assign(
    (selector: (s: typeof mockSearchStoreState) => unknown) => selector(mockSearchStoreState),
    {
      getState: () => mockSearchStoreState,
    }
  ),
}))

const mockShowUndo = vi.fn()
vi.mock('../context/UndoContext', () => ({
  useUndo: () => ({
    showUndo: mockShowUndo,
  }),
}))

describe('Add Page', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.useRealTimers()
  })

  it('renders search input with placeholder and initial empty state', () => {
    render(<Add />)
    expect(screen.getByPlaceholderText('Title name or URL...')).not.toBeNull()
    expect(screen.getByText('Add a title by name or URL')).not.toBeNull()
  })

  it('performs live debounced search across TMDB, AniList, and local titles', async () => {
    vi.mocked(apiFetch).mockImplementation((path: string) => {
      if (path.startsWith('/tmdb/search')) {
        return Promise.resolve([
          { id: 101, title: 'Inception', year: 2010, type: 'movie', poster_url: '/inc.jpg', overview: 'Dreams' },
        ]) as any
      }
      if (path.startsWith('/anilist/search')) {
        return Promise.resolve([
          { id: 202, title: 'Attack on Titan', year: 2013, format: 'TV', poster_url: '/aot.jpg' },
        ]) as any
      }
      if (path.startsWith('/titles?q=')) {
        return Promise.resolve({
          titles: [
            { id: 55, names: [{ name: 'Inception', is_primary: true }], tmdb_id: 101, year: 2010 },
          ],
        }) as any
      }
      return Promise.resolve({}) as any
    })

    render(<Add />)

    const input = screen.getByPlaceholderText('Title name or URL...')
    fireEvent.input(input, { target: { value: 'Incep' } })

    // Fast-forward debounce timer
    vi.advanceTimersByTime(350)

    await waitFor(() => {
      expect(screen.getByText('Inception')).not.toBeNull()
      expect(screen.getByText('Attack on Titan')).not.toBeNull()
    })

    // Inception has localTitleId: 55, so it should display In Library link
    expect(screen.getByText(/In Library/i)).not.toBeNull()

    // Attack on Titan is not in local library, so buttons + Plan to Watch & + Watching should appear
    expect(screen.getByText('+ Plan to Watch')).not.toBeNull()
    expect(screen.getByText('+ Watching')).not.toBeNull()
  })

  it('adds title to library with 1-tap and triggers undo snackbar', async () => {
    vi.mocked(apiFetch).mockImplementation((path: string, options?: any) => {
      if (path.startsWith('/tmdb/search')) {
        return Promise.resolve([
          { id: 999, title: 'Interstellar', year: 2014, type: 'movie', poster_url: '/inter.jpg' },
        ]) as any
      }
      if (path.startsWith('/anilist/search')) {
        return Promise.resolve([]) as any
      }
      if (path.startsWith('/titles?q=')) {
        return Promise.resolve({ titles: [] }) as any
      }
      if (path === '/titles' && options?.method === 'POST') {
        const body = JSON.parse(options.body)
        return Promise.resolve({
          id: 777,
          type: body.type,
          year: body.year,
          status: body.status,
          names: body.names,
        }) as any
      }
      return Promise.resolve({}) as any
    })

    render(<Add />)

    const input = screen.getByPlaceholderText('Title name or URL...')
    fireEvent.input(input, { target: { value: 'Interstellar' } })

    vi.advanceTimersByTime(350)

    await waitFor(() => {
      expect(screen.getByText('Interstellar')).not.toBeNull()
    })

    const planBtn = screen.getByText('+ Plan to Watch')
    fireEvent.click(planBtn)

    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith(
        '/titles',
        expect.objectContaining({
          method: 'POST',
        })
      )
      expect(mockShowUndo).toHaveBeenCalledWith(
        expect.objectContaining({
          message: expect.stringContaining('Interstellar'),
        })
      )
      // Once added, the card shows In Library
      expect(screen.getByText(/In Library/i)).not.toBeNull()
    })
  })
})
