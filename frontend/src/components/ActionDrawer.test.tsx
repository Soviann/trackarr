import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { ActionDrawer } from './ActionDrawer'
import type { Title } from '../types'

function makeTitle(): Title {
  return {
    id: 42,
    type: 'tv',
    status: 'ongoing',
    is_anime: false,
    tmdb_id: null,
    imdb_id: null,
    anilist_id: null,
    tvdb_id: null,
    match_status: 'matched',
    match_source: 'tmdb',
    names: [{ id: 1, title_id: 42, name: 'My Show', language: 'en', is_primary: true }],
    seasons: [],
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

function renderDrawer(overrides: Partial<Parameters<typeof ActionDrawer>[0]> = {}) {
  return render(
    <ActionDrawer
      title={makeTitle()}
      onRate={vi.fn()}
      onEdit={vi.fn()}
      onRematch={vi.fn()}
      onMerge={vi.fn()}
      onRefresh={vi.fn().mockResolvedValue(undefined)}
      onDelete={vi.fn()}
      {...overrides}
    />,
  )
}

describe('ActionDrawer — Delete action', () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('fires onDelete when the Delete button in the More sheet is clicked', () => {
    const onDelete = vi.fn()
    const { getByText } = renderDrawer({ onDelete })

    // Expand the drawer, then open the More sheet where Delete lives.
    fireEvent.click(getByText('Actions'))
    fireEvent.click(getByText('More'))
    fireEvent.click(getByText('Delete'))

    expect(onDelete).toHaveBeenCalledTimes(1)
  })
})
