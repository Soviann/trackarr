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

  it('links the entry name to that season AniList page (mapped variant)', () => {
    const season = makeSeason({ anilist_id: '12345' })
    const { getByText } = render(<SeasonAniListStrip season={season} onEdit={vi.fn()} />)
    const link = getByText('S2').closest('a')
    expect(link?.getAttribute('href')).toBe('https://anilist.co/anime/12345')
    expect(link?.getAttribute('target')).toBe('_blank')
    expect(link?.getAttribute('rel')).toBe('noopener noreferrer')
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

  // anilist_parts multi-part tests
  it('renders two rows with "Part N" tag + "View on AniList" anchor when anilist_parts has two entries', () => {
    const season = makeSeason({
      anilist_parts: [
        { external_id: '111', score: 82, episode_count: 12, start_date: null, sort_order: 1 },
        { external_id: '222', score: 90, episode_count: 13, start_date: null, sort_order: 2 },
      ],
    })
    const { getAllByText, getByText } = render(<SeasonAniListStrip season={season} onEdit={vi.fn()} />)
    // "Part N" appears as tag spans only (not as anchor text)
    expect(getByText('Part 1')).toBeTruthy()
    expect(getByText('Part 2')).toBeTruthy()
    // Anchor text is "View on AniList" (x2 rows)
    const viewLinks = getAllByText('View on AniList')
    expect(viewLinks).toHaveLength(2)
    // Verify links point to correct AniList URLs
    expect(viewLinks[0].getAttribute('href')).toBe('https://anilist.co/anime/111')
    expect(viewLinks[1].getAttribute('href')).toBe('https://anilist.co/anime/222')
    // "Part 1" / "Part 2" are NOT anchor elements (they are spans)
    expect(getByText('Part 1').tagName).not.toBe('A')
    expect(getByText('Part 2').tagName).not.toBe('A')
    // Scores rendered
    expect(getByText('82%')).toBeTruthy()
    expect(getByText('90%')).toBeTruthy()
  })

  it('renders single-link look (no "Part" label) when anilist_parts has one entry', () => {
    const season = makeSeason({
      anilist_parts: [
        { external_id: '999', score: 75, episode_count: 12, start_date: null, sort_order: 1 },
      ],
    })
    const { getByText, queryByText } = render(<SeasonAniListStrip season={season} onEdit={vi.fn()} />)
    // Should show S2 (season fallback) not "Part 1"
    expect(getByText('S2')).toBeTruthy()
    expect(queryByText('Part 1')).toBeNull()
    expect(getByText('75%')).toBeTruthy()
  })

  it('renders the unmapped state when anilist_parts is empty and no anilist_id', () => {
    const season = makeSeason({ anilist_parts: [], anilist_id: null })
    const { getByText } = render(<SeasonAniListStrip season={season} onEdit={vi.fn()} />)
    expect(getByText('Not mapped for this season')).toBeTruthy()
    expect(getByText('Link entry')).toBeTruthy()
  })
})
