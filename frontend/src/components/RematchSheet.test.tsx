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
    mockApiFetch.mockResolvedValue(undefined)
  })

  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
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

    const { getByPlaceholderText, getByText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={onDone} />,
    )

    const input = getByPlaceholderText('e.g. 145064')
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

  it('calls DELETE /titles/{id}/seasons/{seasonID}/anilist/{externalID} when Remove is clicked', async () => {
    const parts = [makePart('145064', null)]
    const season = makeSeason(10, parts)
    const title = makeTitle([season])
    const onDone = vi.fn()

    const { getByText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={onDone} />,
    )

    const removeBtn = getByText('Remove')
    fireEvent.click(removeBtn)

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/titles/42/seasons/10/anilist/145064',
        expect.objectContaining({ method: 'DELETE' }),
      )
      expect(onDone).toHaveBeenCalled()
    })
  })

  it('calls PUT .../anilist/order with swapped ordered_ids when ▼ down button is clicked', async () => {
    const parts = [makePart('145064', 78), makePart('145065', 90)]
    const season = makeSeason(10, parts)
    const title = makeTitle([season])
    const onDone = vi.fn()

    const { getAllByLabelText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={onDone} />,
    )

    // "Move part 1 down" moves index 0 → swap with index 1
    const moveDownBtn = getAllByLabelText('Move part 1 down')[0]
    fireEvent.click(moveDownBtn)

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/titles/42/seasons/10/anilist/order',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ ordered_ids: ['145065', '145064'] }),
        }),
      )
      expect(onDone).toHaveBeenCalled()
    })
  })

  it('calls PUT .../anilist/order with swapped ordered_ids when ▲ up button is clicked', async () => {
    const parts = [makePart('145064', 78), makePart('145065', 90)]
    const season = makeSeason(10, parts)
    const title = makeTitle([season])
    const onDone = vi.fn()

    const { getAllByLabelText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={onDone} />,
    )

    // "Move part 2 up" moves index 1 → swap with index 0
    const moveUpBtn = getAllByLabelText('Move part 2 up')[0]
    fireEvent.click(moveUpBtn)

    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/titles/42/seasons/10/anilist/order',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify({ ordered_ids: ['145065', '145064'] }),
        }),
      )
      expect(onDone).toHaveBeenCalled()
    })
  })

  it('disables ▲ on first row and ▼ on last row', () => {
    const parts = [makePart('145064', 78), makePart('145065', 90)]
    const season = makeSeason(10, parts)
    const title = makeTitle([season])

    const { getByLabelText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={vi.fn()} />,
    )

    expect((getByLabelText('Move part 1 up') as HTMLButtonElement).disabled).toBe(true)
    expect((getByLabelText('Move part 1 down') as HTMLButtonElement).disabled).toBe(false)
    expect((getByLabelText('Move part 2 up') as HTMLButtonElement).disabled).toBe(false)
    expect((getByLabelText('Move part 2 down') as HTMLButtonElement).disabled).toBe(true)
  })

  it('does not show reorder buttons when only one part', () => {
    const parts = [makePart('145064', 78)]
    const season = makeSeason(10, parts)
    const title = makeTitle([season])

    const { queryByLabelText } = render(
      <RematchSheet open={true} onClose={vi.fn()} title={title} seasonID={10} onDone={vi.fn()} />,
    )

    expect(queryByLabelText('Move part 1 up')).toBeNull()
    expect(queryByLabelText('Move part 1 down')).toBeNull()
  })

  it('does not call onClose after add — sheet stays open', async () => {
    const season = makeSeason(10, [])
    const title = makeTitle([season])
    const onClose = vi.fn()
    const onDone = vi.fn()

    const { getByPlaceholderText, getByText } = render(
      <RematchSheet open={true} onClose={onClose} title={title} seasonID={10} onDone={onDone} />,
    )

    fireEvent.input(getByPlaceholderText('e.g. 145064'), { target: { value: '99999' } })
    fireEvent.click(getByText('Add'))

    await waitFor(() => expect(onDone).toHaveBeenCalled())
    expect(onClose).not.toHaveBeenCalled()
  })
})
