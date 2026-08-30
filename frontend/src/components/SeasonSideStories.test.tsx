import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { SeasonSideStories } from './SeasonSideStories'
import type { TitleRelation } from '../types'

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
    title: 'My Hero Academia: Two Heroes',
    romaji_title: 'Boku no Hero Academia the Movie: Futari no Hero',
    year: 2018,
    score: 82,
    duration: 96,
    overview: 'Deku and All Might visit I-Island.',
    sort_order: 1,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('SeasonSideStories', () => {
  it('renders nothing when sideStories is empty', () => {
    const { container } = render(<SeasonSideStories seasonNumber={2} sideStories={[]} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders a single side story card with movie info', () => {
    const rel = makeRelation()
    const { getByText } = render(<SeasonSideStories seasonNumber={2} sideStories={[rel]} />)

    expect(getByText('Side Story recommandée après la Saison 2')).toBeTruthy()
    expect(getByText('Film')).toBeTruthy()
    expect(getByText('My Hero Academia: Two Heroes')).toBeTruthy()
    expect(getByText('2018')).toBeTruthy()
    expect(getByText('· 1h 36m')).toBeTruthy()
    expect(getByText('★ 82%')).toBeTruthy()
  })

  it('renders plural header when multiple side stories exist for the season', () => {
    const rel1 = makeRelation({ id: 1, external_id: 101347, title: 'Two Heroes' })
    const rel2 = makeRelation({ id: 2, external_id: 98565, title: 'Training of the Dead', format: 'OVA' })

    const { getByText } = render(
      <SeasonSideStories seasonNumber={2} sideStories={[rel1, rel2]} />
    )

    expect(getByText('Side Stories recommandées après la Saison 2 (2)')).toBeTruthy()
    expect(getByText('Two Heroes')).toBeTruthy()
    expect(getByText('Training of the Dead')).toBeTruthy()
    expect(getByText('OAV')).toBeTruthy()
  })

  it('shows matched title watched button and calls onToggleWatched when clicked', () => {
    const rel = makeRelation({
      matched_title_id: 42,
      matched_status: 'completed',
    })
    const onToggle = vi.fn()
    const { getByText } = render(
      <SeasonSideStories seasonNumber={2} sideStories={[rel]} onToggleWatched={onToggle} />
    )

    const btn = getByText('Vu (Trackarr)')
    expect(btn).toBeTruthy()
    fireEvent.click(btn)
    expect(onToggle).toHaveBeenCalledWith(rel)
  })

  it('shows unmatched AniList link and + Ajouter button when title is not in library', () => {
    const rel = makeRelation({
      matched_title_id: null,
      external_id: 101347,
    })
    const { getByText } = render(
      <SeasonSideStories seasonNumber={2} sideStories={[rel]} />
    )

    expect(getByText('Non présent')).toBeTruthy()
    expect(getByText('Ajouter')).toBeTruthy()
    const link = getByText('AniList').closest('a')
    expect(link?.getAttribute('href')).toBe('https://anilist.co/anime/101347')
  })
})
