import { describe, it, expect, afterEach, vi, beforeEach } from 'vitest'
import { render, fireEvent, waitFor, screen, cleanup } from '@testing-library/preact'
import { FranchiseRelationsSection } from './FranchiseRelationsSection'
import { apiFetch } from '../api'
import type { TitleRelation } from '../types'

const mockInvalidate = vi.fn()

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

vi.mock('../store', () => ({
  useTitleStore: (selector?: (st: { invalidate: typeof mockInvalidate }) => unknown) => {
    const state = { invalidate: mockInvalidate }
    return selector ? selector(state) : state
  },
}))

beforeEach(() => {
  vi.resetAllMocks()
})

afterEach(() => cleanup())

function makeRelation(overrides: Partial<TitleRelation> = {}): TitleRelation {
  return {
    id: 1,
    title_id: 10,
    season_id: 2,
    season_number: 2,
    provider: 'anilist',
    external_id: 101347,
    relation_type: 'SIDE_STORY',
    format: 'MOVIE',
    title: 'Two Heroes',
    romaji_title: 'Boku no Hero Academia the Movie: Futari no Hero',
    year: 2018,
    score: 82,
    duration: 96,
    overview: 'All Might and Deku.',
    sort_order: 1,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('FranchiseRelationsSection', () => {
  it('renders nothing when relations list is empty and no onOpenHistory', () => {
    const { container } = render(<FranchiseRelationsSection relations={[]} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders combined hub bar with both franchise row and history row', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 101, format: 'MOVIE', title: 'Movie 1', matched_status: 'completed' }),
      makeRelation({ id: 2, external_id: 102, format: 'TV', title: 'Series 1' }),
    ]
    const onOpenHistory = vi.fn()
    const { getByText } = render(
      <FranchiseRelationsSection relations={rels} onOpenHistory={onOpenHistory} />
    )

    // Franchise glance row
    expect(getByText('AniList Relations')).toBeTruthy()
    expect(getByText('(2)')).toBeTruthy()
    expect(getByText('1 / 2 Titles seen')).toBeTruthy()
    expect(getByText(/Series 1/)).toBeTruthy()
    expect(getByText('50%')).toBeTruthy()

    // History glance row
    const historyBtn = getByText('Watch History')
    expect(historyBtn).toBeTruthy()
    fireEvent.click(historyBtn)
    expect(onOpenHistory).toHaveBeenCalledOnce()
  })

  it('opens drawer on franchise row click and renders filter tabs', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 101, format: 'MOVIE', title: 'Movie 1' }),
      makeRelation({ id: 2, external_id: 102, format: 'OVA', title: 'OVA 1' }),
      makeRelation({ id: 3, external_id: 103, relation_type: 'SPIN_OFF', format: 'TV', title: 'Spin-off 1' }),
    ]
    const { getByText, getAllByText } = render(
      <FranchiseRelationsSection relations={rels} />
    )

    // Initially drawer is closed; open it by clicking the hub row
    fireEvent.click(getByText('AniList Relations'))

    expect(getAllByText('AniList Relations').length).toBeGreaterThanOrEqual(1)
    expect(getByText('All (3)')).toBeTruthy()
    expect(getByText('Movies (1)')).toBeTruthy()
    expect(getByText('OVAs (1)')).toBeTruthy()
    expect(getByText('Spin-offs (1)')).toBeTruthy()
  })

  it('filters relations when clicking category tabs inside drawer', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 101, format: 'MOVIE', title: 'Movie 1' }),
      makeRelation({ id: 2, external_id: 102, format: 'OVA', title: 'OVA 1' }),
    ]
    const { getAllByText, getByText } = render(
      <FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />
    )

    expect(getAllByText(/Movie 1/).length).toBeGreaterThanOrEqual(1)
    expect(getAllByText(/OVA 1/).length).toBeGreaterThanOrEqual(1)

    // Filter to movies
    fireEvent.click(getByText('Movies (1)'))
    expect(getAllByText(/Movie 1/).length).toBeGreaterThanOrEqual(1)
    expect(document.querySelector('.itemTitle')?.textContent).toContain('Movie 1')

    // Filter to OVAs
    fireEvent.click(getByText('OVAs (1)'))
    expect(document.querySelector('.itemTitle')?.textContent).toContain('OVA 1')

    // Back to All
    fireEvent.click(getByText('All (2)'))
    expect(getAllByText(/Movie 1/).length).toBeGreaterThanOrEqual(1)
    expect(getAllByText(/OVA 1/).length).toBeGreaterThanOrEqual(1)
  })

  it('shows matched watch status badge correctly inside drawer', () => {
    const rels = [
      makeRelation({ id: 1, matched_title_id: 50, matched_status: 'completed', title: 'Movie Vu' }),
      makeRelation({ id: 2, matched_title_id: 51, matched_status: 'watching', title: 'Movie En cours' }),
      makeRelation({ id: 3, matched_title_id: null, title: 'Movie Absent' }),
    ]
    const { getByText } = render(
      <FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />
    )

    expect(getByText('✓ Watched (Trackarr)')).toBeTruthy()
    expect(getByText('Plan to Watch')).toBeTruthy()
    expect(getByText('+ To Watch')).toBeTruthy()
  })

  it('renders TMDB movie saga collection with Saga & Collection title', () => {
    const rels = [
      makeRelation({ id: 1, provider: 'tmdb', relation_type: 'COLLECTION', external_id: 671, format: 'MOVIE', title: 'Harry Potter 1', year: 2001 }),
      makeRelation({ id: 2, provider: 'tmdb', relation_type: 'COLLECTION', external_id: 672, format: 'MOVIE', title: 'Harry Potter 2', year: 2002 }),
    ]
    const { getAllByText, getByText } = render(
      <FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />
    )

    expect(getAllByText('TMDB Saga').length).toBeGreaterThanOrEqual(1)
    expect(getAllByText(/Harry Potter 1/).length).toBeGreaterThanOrEqual(1)
    expect(getAllByText(/Harry Potter 2/).length).toBeGreaterThanOrEqual(1)
  })

  it('toggles sort order between timeline and release date', () => {
    const rels = [
      makeRelation({ id: 1, sort_order: 2, year: 2010, title: 'B Movie' }),
      makeRelation({ id: 2, sort_order: 1, year: 2020, title: 'A Movie' }),
    ]
    const { getByText } = render(
      <FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />
    )

    // Toggle to Release Date
    const releaseBtn = getByText('📅 Release')
    fireEvent.click(releaseBtn)
    expect(releaseBtn.className).toContain('sortBtnActive')

    // Toggle back to Timeline
    const timelineBtn = getByText('⏱️ Timeline')
    fireEvent.click(timelineBtn)
    expect(timelineBtn.className).toContain('sortBtnActive')
  })

  it('collapses by default above 3 items and expands when clicking Show more in drawer', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 1, title: 'Item 1' }),
      makeRelation({ id: 2, external_id: 2, title: 'Item 2' }),
      makeRelation({ id: 3, external_id: 3, title: 'Item 3' }),
      makeRelation({ id: 4, external_id: 4, title: 'Item 4' }),
      makeRelation({ id: 5, external_id: 5, title: 'Item 5' }),
    ]
    const { getAllByText, getByText } = render(
      <FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />
    )

    // Items 1, 2, 3 visible in grid
    expect(getAllByText(/Item 1/).length).toBeGreaterThanOrEqual(1)
    expect(getAllByText(/Item 2/).length).toBeGreaterThanOrEqual(1)
    expect(getAllByText(/Item 3/).length).toBeGreaterThanOrEqual(1)

    // Button shows "Show more (+2)"
    const toggleBtn = getByText('Show more (+2)')
    expect(toggleBtn).toBeTruthy()

    // Expand
    fireEvent.click(toggleBtn)
    expect(getAllByText(/Item 4/).length).toBeGreaterThanOrEqual(1)
    expect(getAllByText(/Item 5/).length).toBeGreaterThanOrEqual(1)
    expect(getByText('Show less')).toBeTruthy()

    // Collapse back
    fireEvent.click(getByText('Show less'))
    expect(getByText('Show more (+2)')).toBeTruthy()
  })

  it('renders progress and next chronological title in hub bar and drawer', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 101, title: 'Iron Man', year: 2008, sort_order: 1, matched_status: 'completed' }),
      makeRelation({ id: 2, external_id: 102, title: 'Iron Man 2', year: 2010, sort_order: 2, matched_status: 'completed' }),
      makeRelation({ id: 3, external_id: 103, title: 'The Avengers', year: 2012, sort_order: 3, matched_status: 'watching' }),
      makeRelation({ id: 4, external_id: 104, title: 'Iron Man 3', year: 2013, sort_order: 4 }),
    ]
    const { getAllByText, getByText } = render(
      <FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />
    )

    // 2 out of 4 titles seen
    expect(getAllByText('2 / 4 Titles seen').length).toBeGreaterThanOrEqual(1)
    // Next chronological title
    expect(getAllByText(/The Avengers/).length).toBeGreaterThanOrEqual(1)
  })

  it('displays completion message when all titles are seen', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 101, title: 'Movie 1', matched_status: 'completed' }),
      makeRelation({ id: 2, external_id: 102, title: 'Movie 2', matched_status: 'completed' }),
    ]
    const { getAllByText, getByText } = render(
      <FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />
    )

    expect(getAllByText('2 / 2 Titles seen').length).toBeGreaterThanOrEqual(1)
    expect(getByText('All titles completed!')).toBeTruthy()
  })

  it('applies accentHex custom CSS variables to hub bar and drawer container', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 101, title: 'Iron Man', year: 2008 }),
    ]
    const { container } = render(
      <FranchiseRelationsSection relations={rels} accentHex="#e11d48" initialOpenDrawer={true} />
    )

    const hubBar = container.querySelector('[class*="hubBar"]') as HTMLElement
    expect(hubBar).toBeTruthy()
    expect(hubBar.style.getPropertyValue('--cover-accent')).toBe('#e11d48')
    expect(hubBar.style.getPropertyValue('--cover-accent-wash')).toBe('rgba(225, 29, 72, 0.12)')
    expect(hubBar.style.getPropertyValue('--cover-accent-glow')).toBe('rgba(225, 29, 72, 0.24)')

    const drawerContainer = document.querySelector('[class*="drawerContainer"]') as HTMLElement
    expect(drawerContainer).toBeTruthy()
    expect(drawerContainer.style.getPropertyValue('--cover-accent')).toBe('#e11d48')
  })

  it('clears matched_radarr_id, calls invalidate, and closes ArrPushSheet on movie deletion', async () => {
    const rels = [
      makeRelation({
        id: 1,
        provider: 'tmdb',
        external_id: 603,
        format: 'MOVIE',
        title: 'The Matrix',
        matched_title_id: 50,
        matched_status: 'completed',
        matched_type: 'movie',
        matched_radarr_id: 777,
      }),
    ]

    vi.mocked(apiFetch).mockImplementation(async (path: string, init?: RequestInit) => {
      if (path === '/titles/50') {
        return {
          id: 50,
          type: 'movie',
          is_anime: false,
          year: 1999,
          cover_url: null,
          accent_hex: null,
          imdb_id: null,
          simkl_id: null,
          simkl_slug: null,
          anilist_id: null,
          tmdb_id: 603,
          tvdb_id: null,
          radarr_id: 777,
          downloaded_at: '2026-01-01T00:00:00Z',
          total_watch_minutes: 0,
          my_rating: 9,
          status: 'completed',
          series_status: null,
          match_status: 'confirmed',
          original_title: 'The Matrix',
          match_source: null,
          overview: null,
          genres: null,
          runtime: 136,
          tmdb_rating: null,
          credits: null,
          anilist_rating: null,
          release_date: null,
          created_at: '',
          updated_at: '',
          names: [{ id: 1, title_id: 50, name: 'The Matrix', language: 'en', is_primary: true }],
          seasons: [],
        }
      }
      if (path === '/admin/arr') return { radarr_std_root_folder: '/movies', radarr_std_quality_profile: '1' }
      if (path.includes('/rootfolder')) return [{ id: 1, path: '/movies' }]
      if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD' }]
      if (path === '/arr/title/50' && (!init || !init.method || init.method === 'GET')) {
        return {
          app: 'radarr',
          exists: true,
          arr_id: 777,
          monitored: true,
          quality_profile_id: 1,
          root_folder_path: '/movies',
          has_file: true,
        }
      }
      if (path === '/arr/title/50' && init?.method === 'DELETE') {
        return { status: 'ok' }
      }
      return {}
    })

    render(<FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />)

    fireEvent.click(screen.getByRole('button', { name: 'Send to Arr' }))

    const deleteBtn = await screen.findByRole('button', { name: /Delete from Radarr/i })
    fireEvent.click(deleteBtn)
    fireEvent.click(screen.getByRole('button', { name: /Confirm permanent deletion/i }))

    await waitFor(() => {
      expect(mockInvalidate).toHaveBeenCalledOnce()
      expect(screen.queryByRole('button', { name: /Confirm permanent deletion/i })).toBeNull()
    })

    expect(apiFetch).toHaveBeenCalledWith(
      '/arr/title/50',
      expect.objectContaining({ method: 'DELETE' })
    )
  })

  it('calls invalidate and keeps ArrPushSheet open with stale notice when matched title is unlinked on open', async () => {
    const rels = [
      makeRelation({
        id: 1,
        provider: 'tmdb',
        external_id: 603,
        format: 'MOVIE',
        title: 'The Matrix',
        matched_title_id: 50,
        matched_status: 'completed',
        matched_type: 'movie',
        matched_radarr_id: 777,
      }),
    ]

    vi.mocked(apiFetch).mockImplementation(async (path: string) => {
      if (path === '/titles/50') {
        return {
          id: 50,
          type: 'movie',
          is_anime: false,
          year: 1999,
          cover_url: null,
          accent_hex: null,
          imdb_id: null,
          simkl_id: null,
          simkl_slug: null,
          anilist_id: null,
          tmdb_id: 603,
          tvdb_id: null,
          radarr_id: 777,
          total_watch_minutes: 0,
          my_rating: 9,
          status: 'completed',
          series_status: null,
          match_status: 'confirmed',
          original_title: 'The Matrix',
          match_source: null,
          overview: null,
          genres: null,
          runtime: 136,
          tmdb_rating: null,
          credits: null,
          anilist_rating: null,
          release_date: null,
          created_at: '',
          updated_at: '',
          names: [{ id: 1, title_id: 50, name: 'The Matrix', language: 'en', is_primary: true }],
          seasons: [],
        }
      }
      if (path === '/admin/arr') return { radarr_std_root_folder: '/movies', radarr_std_quality_profile: '1' }
      if (path.includes('/rootfolder')) return [{ id: 1, path: '/movies' }]
      if (path.includes('/qualityprofile')) return [{ id: 1, name: 'HD' }]
      if (path === '/arr/title/50') return { app: 'radarr', exists: false }
      return {}
    })

    render(<FranchiseRelationsSection relations={rels} initialOpenDrawer={true} />)

    fireEvent.click(screen.getByRole('button', { name: 'Send to Arr' }))

    await waitFor(() => {
      expect(mockInvalidate).toHaveBeenCalledOnce()
      expect(screen.getByText(/was not found in Radarr/i)).toBeTruthy()
      expect(screen.getByText('Send to Radarr')).toBeTruthy()
    })
  })
})
