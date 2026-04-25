import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { SeasonAniListStrip } from './SeasonAniListStrip'
import type { Season } from '../types'

afterEach(() => cleanup())

function makeSeason(overrides: Partial<Season> = {}): Season {
  return {
    id: 1,
    title_id: 10,
    season_number: 2,
    total_episodes: 13,
    episodes: [],
    ...overrides,
  }
}

describe('SeasonAniListStrip', () => {
  it('renders the mapped variant when anilist_id is set, with score', () => {
    const season = makeSeason({ anilist_id: '12345', anilist_community_score: 87 })
    const { getByText, getByLabelText } = render(
      <SeasonAniListStrip season={season} onEdit={vi.fn()} />,
    )
    expect(getByText('ANILIST')).toBeTruthy()
    expect(getByText('S2')).toBeTruthy()
    expect(getByText('87%')).toBeTruthy()
    expect(getByLabelText('Edit AniList mapping')).toBeTruthy()
  })

  it('renders the unmapped variant when anilist_id is null', () => {
    const season = makeSeason({ anilist_id: null })
    const { getByText } = render(<SeasonAniListStrip season={season} onEdit={vi.fn()} />)
    expect(getByText('Not mapped for this season')).toBeTruthy()
    expect(getByText('Link entry')).toBeTruthy()
  })

  it('hides the score when anilist_community_score is null but anilist_id is set', () => {
    const season = makeSeason({ anilist_id: '12345', anilist_community_score: null })
    const { queryByText } = render(<SeasonAniListStrip season={season} onEdit={vi.fn()} />)
    expect(queryByText(/%$/)).toBeNull()
  })

  it('uses entryName when provided instead of the season fallback', () => {
    const season = makeSeason({ anilist_id: '12345' })
    const { getByText } = render(
      <SeasonAniListStrip season={season} entryName="Solo Leveling S2" onEdit={vi.fn()} />,
    )
    expect(getByText('Solo Leveling S2')).toBeTruthy()
  })

  it('fires onEdit when the edit button is clicked (mapped variant)', () => {
    const onEdit = vi.fn()
    const season = makeSeason({ anilist_id: '12345' })
    const { getByLabelText } = render(<SeasonAniListStrip season={season} onEdit={onEdit} />)
    fireEvent.click(getByLabelText('Edit AniList mapping'))
    expect(onEdit).toHaveBeenCalledOnce()
  })

  it('fires onEdit when the link button is clicked (unmapped variant)', () => {
    const onEdit = vi.fn()
    const season = makeSeason({ anilist_id: null })
    const { getByText } = render(<SeasonAniListStrip season={season} onEdit={onEdit} />)
    fireEvent.click(getByText('Link entry'))
    expect(onEdit).toHaveBeenCalledOnce()
  })
})
