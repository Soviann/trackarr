import { describe, it, expect, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { FranchiseRelationsSection } from './FranchiseRelationsSection'
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
  it('renders nothing when relations list is empty', () => {
    const { container } = render(<FranchiseRelationsSection relations={[]} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders section title, count and filter tabs', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 101, format: 'MOVIE', title: 'Movie 1' }),
      makeRelation({ id: 2, external_id: 102, format: 'OVA', title: 'OVA 1' }),
      makeRelation({ id: 3, external_id: 103, relation_type: 'SPIN_OFF', format: 'TV', title: 'Spin-off 1' }),
    ]
    const { getByText } = render(<FranchiseRelationsSection relations={rels} />)

    expect(getByText('Univers & Franchise')).toBeTruthy()
    expect(getByText('Tous (3)')).toBeTruthy()
    expect(getByText('Films (1)')).toBeTruthy()
    expect(getByText('OAVs (1)')).toBeTruthy()
    expect(getByText('Spin-offs (1)')).toBeTruthy()
  })

  it('filters relations when clicking category tabs', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 101, format: 'MOVIE', title: 'Movie 1' }),
      makeRelation({ id: 2, external_id: 102, format: 'OVA', title: 'OVA 1' }),
    ]
    const { getByText, queryByText } = render(<FranchiseRelationsSection relations={rels} />)

    expect(getByText('Movie 1')).toBeTruthy()
    expect(getByText('OVA 1')).toBeTruthy()

    // Filter to movies
    fireEvent.click(getByText('Films (1)'))
    expect(getByText('Movie 1')).toBeTruthy()
    expect(queryByText('OVA 1')).toBeNull()

    // Filter to OVAs
    fireEvent.click(getByText('OAVs (1)'))
    expect(queryByText('Movie 1')).toBeNull()
    expect(getByText('OVA 1')).toBeTruthy()

    // Back to All
    fireEvent.click(getByText('Tous (2)'))
    expect(getByText('Movie 1')).toBeTruthy()
    expect(getByText('OVA 1')).toBeTruthy()
  })

  it('shows matched watch status badge correctly', () => {
    const rels = [
      makeRelation({ id: 1, matched_title_id: 50, matched_status: 'completed', title: 'Movie Vu' }),
      makeRelation({ id: 2, matched_title_id: 51, matched_status: 'watching', title: 'Movie En cours' }),
      makeRelation({ id: 3, matched_title_id: null, title: 'Movie Absent' }),
    ]
    const { getByText } = render(<FranchiseRelationsSection relations={rels} />)

    expect(getByText('✓ Vu')).toBeTruthy()
    expect(getByText('À voir')).toBeTruthy()
    expect(getByText('+ Ajouter')).toBeTruthy()
  })

  it('renders TMDB movie saga collection with Saga & Collection title', () => {
    const rels = [
      makeRelation({ id: 1, provider: 'tmdb', relation_type: 'COLLECTION', external_id: 671, format: 'MOVIE', title: 'Harry Potter 1', year: 2001 }),
      makeRelation({ id: 2, provider: 'tmdb', relation_type: 'COLLECTION', external_id: 672, format: 'MOVIE', title: 'Harry Potter 2', year: 2002 }),
    ]
    const { getByText } = render(<FranchiseRelationsSection relations={rels} />)

    expect(getByText('Saga & Collection')).toBeTruthy()
    expect(getByText('Saga TMDB')).toBeTruthy()
    expect(getByText('Harry Potter 1')).toBeTruthy()
    expect(getByText('Harry Potter 2')).toBeTruthy()
  })

  it('toggles sort order between timeline and release date', () => {
    const rels = [
      makeRelation({ id: 1, sort_order: 2, year: 2010, title: 'B Movie' }),
      makeRelation({ id: 2, sort_order: 1, year: 2020, title: 'A Movie' }),
    ]
    const { getByText } = render(<FranchiseRelationsSection relations={rels} />)

    // Toggle to Release Date
    const releaseBtn = getByText('📅 Sortie')
    fireEvent.click(releaseBtn)
    expect(releaseBtn.className).toContain('sortBtnActive')

    // Toggle back to Timeline
    const timelineBtn = getByText('⏱️ Chronologie')
    fireEvent.click(timelineBtn)
    expect(timelineBtn.className).toContain('sortBtnActive')
  })

  it('collapses by default above 3 items and expands when clicking Voir plus', () => {
    const rels = [
      makeRelation({ id: 1, external_id: 1, title: 'Item 1' }),
      makeRelation({ id: 2, external_id: 2, title: 'Item 2' }),
      makeRelation({ id: 3, external_id: 3, title: 'Item 3' }),
      makeRelation({ id: 4, external_id: 4, title: 'Item 4' }),
      makeRelation({ id: 5, external_id: 5, title: 'Item 5' }),
    ]
    const { getByText, queryByText } = render(<FranchiseRelationsSection relations={rels} />)

    // Items 1, 2, 3 visible by default
    expect(getByText('Item 1')).toBeTruthy()
    expect(getByText('Item 2')).toBeTruthy()
    expect(getByText('Item 3')).toBeTruthy()
    expect(queryByText('Item 4')).toBeNull()
    expect(queryByText('Item 5')).toBeNull()

    // Button shows "Voir plus (+2)"
    const toggleBtn = getByText('Voir plus (+2)')
    expect(toggleBtn).toBeTruthy()

    // Expand
    fireEvent.click(toggleBtn)
    expect(getByText('Item 4')).toBeTruthy()
    expect(getByText('Item 5')).toBeTruthy()
    expect(getByText('Voir moins')).toBeTruthy()

    // Collapse back
    fireEvent.click(getByText('Voir moins'))
    expect(queryByText('Item 4')).toBeNull()
    expect(queryByText('Item 5')).toBeNull()
    expect(getByText('Voir plus (+2)')).toBeTruthy()
  })
})
