import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup, waitFor } from '@testing-library/preact'
import { RematchSheet } from './RematchSheet'
import type { Title, Season, AniListPart } from '../types'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '../api'

const mockApiFetch = apiFetch as ReturnType<typeof vi.fn>

function makePart(external_id: string, score: number | null = null): AniListPart {
  return { external_id, score, episode_count: null, start_date: null, sort_order: null }
}

function makeSeason(id: number, anilist_parts: AniListPart[] = []): Season {
  return {
    id,
    title_id: 1,
    season_number: 1,
    total_episodes: 24,
    anilist_parts,
    episodes: [],
  }
}

function makeTitle(seasons: Season[] = []): Title {
  return {
    id: 42,
    type: 'tv',
    status: 'ongoing',
    is_anime: true,
    tmdb_id: null,
    imdb_id: null,
    anilist_id: null,
    tvdb_id: null,
    match_status: 'matched',
    match_source: 'anilist',
    names: [{ id: 1, title_id: 42, name: 'My Anime', language: 'en', is_primary: true }],
    seasons,
    match_result: null,
    genres: [],
    tags: [],
    watched_count: 0,
    episode_count: 0,
    plex_key: null,
    added_at: '2024-01-01',
    updated_at: '2024-01-01',
  } as unknown as Title
}

describe('RematchSheet — season mode (multi-link AniList manager)', () => {
  beforeEach(() => {
    mockApiFetch.mockResolvedValue([])
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('automatically searches AniList on open with title name', async () => {
    const season = makeSeason(10, [])
    const title = makeTitle([season])

    render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={vi.fn()} />,
    )

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith('/anilist/search?query=My%20Anime')
    })
  })

  it('renders search results and links anime on click', async () => {
    mockApiFetch.mockImplementation(async (url: string) => {
      if (url.startsWith('/anilist/search')) {
        return [
          {
            id: 154587,
            romaji_title: 'Sousou no Frieren',
            english_title: "Frieren: Beyond Journey's End",
            title: "Frieren: Beyond Journey's End",
            year: 2023,
            format: 'TV',
            episodes: 28,
            poster_url: 'https://example.com/poster.jpg',
          },
        ]
      }
      return undefined
    })

    const season = makeSeason(10, [])
    const title = makeTitle([season])
    const onDone = vi.fn()
    const onClose = vi.fn()

    const { getByText } = render(
      <RematchSheet open={true} onClose={onClose} title={title} seasonID={10} onDone={onDone} />,
    )

    await waitFor(() => {
      expect(getByText("Frieren: Beyond Journey's End")).toBeTruthy()
      expect(getByText("Sousou no Frieren")).toBeTruthy()
      expect(getByText("#154587")).toBeTruthy()
    })

    fireEvent.click(getByText("Frieren: Beyond Journey's End"))

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/titles/42/seasons/10/anilist',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ anilist_id: '154587' }),
        }),
      )
      expect(onDone).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })

  it('renders the external AniList search link with prefilled query', () => {
    const season = makeSeason(10, [])
    const title = makeTitle([season])

    const { getByText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={vi.fn()} />,
    )

    const link = getByText(/Search on AniList\.co/i).closest('a')
    expect(link).toBeTruthy()
    expect(link?.getAttribute('href')).toBe('https://anilist.co/search/anime?search=My%20Anime')
    expect(link?.getAttribute('target')).toBe('_blank')
  })

  it('lists both parts with Remove buttons when season has two anilist_parts', () => {
    const parts = [makePart('145064', 78), makePart('145065', null)]
    const season = makeSeason(10, parts)
    const title = makeTitle([season])

    const { getByText, getAllByText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={vi.fn()} />,
    )

    expect(getByText(/Part 1:/)).toBeTruthy()
    expect(getByText(/145064/)).toBeTruthy()
    expect(getByText(/78%/)).toBeTruthy()
    expect(getByText(/Part 2:/)).toBeTruthy()
    expect(getByText(/145065/)).toBeTruthy()
    const removeButtons = getAllByText('Remove')
    expect(removeButtons).toHaveLength(2)
  })

  it('calls POST /titles/{id}/seasons/{seasonID}/anilist with anilist_id when Add is clicked', async () => {
    const season = makeSeason(10, [])
    const title = makeTitle([season])
    const onDone = vi.fn()

    const { getByText, getByPlaceholderText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={onDone} />,
    )

    // Expand manual section
    fireEvent.click(getByText(/Manual AniList ID/i))

    const input = getByPlaceholderText('e.g. 26 or https://anilist.co/anime/26')
    fireEvent.input(input, { target: { value: '145064' } })

    const addBtn = getByText('Add')
    fireEvent.click(addBtn)

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/titles/42/seasons/10/anilist',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ anilist_id: '145064' }),
        }),
      )
      expect(onDone).toHaveBeenCalled()
    })
  })

  it('extracts numeric AniList ID from full URL when Add is clicked', async () => {
    const season = makeSeason(10, [])
    const title = makeTitle([season])
    const onDone = vi.fn()
    const onClose = vi.fn()

    const { getByText, getByPlaceholderText } = render(
      <RematchSheet open={true} onClose={onClose} title={title} seasonID={10} onDone={onDone} />,
    )

    fireEvent.click(getByText(/Manual AniList ID/i))

    const input = getByPlaceholderText('e.g. 26 or https://anilist.co/anime/26')
    fireEvent.input(input, { target: { value: 'https://anilist.co/anime/26' } })

    const addBtn = getByText('Add')
    fireEvent.click(addBtn)

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/titles/42/seasons/10/anilist',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ anilist_id: '26' }),
        }),
      )
      expect(onDone).toHaveBeenCalled()
      expect(onClose).toHaveBeenCalled()
    })
  })
})
