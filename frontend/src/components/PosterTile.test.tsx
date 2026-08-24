import { describe, it, expect } from 'vitest'
import { render } from '@testing-library/preact'
import { PosterTile, type PosterTileItem } from './PosterTile'

describe('PosterTile', () => {
  it('renders type badge with Sonarr indicator for series', () => {
    const item: PosterTileItem = {
      id: 1,
      type: 'series',
      sonarr_id: 123,
      cover_url: null,
      name: 'Severance',
      sublabel: 'S02E01',
      progressRatio: 0.5,
    }

    const { container } = render(<PosterTile item={item} />)
    const badge = container.querySelector('[aria-label="Series (Tracked on Sonarr)"]')
    expect(badge).not.toBeNull()
  })

  it('renders PrimeBadge when onPrime is true', () => {
    const item: PosterTileItem = {
      id: 2,
      type: 'movie',
      radarr_id: 456,
      cover_url: null,
      name: 'Interstellar',
      sublabel: 'Available',
      onPrime: true,
    }

    const { container, getByText } = render(<PosterTile item={item} />)
    expect(getByText('prime')).not.toBeNull()
    const badge = container.querySelector('[aria-label="Movie (Tracked on Radarr)"]')
    expect(badge).not.toBeNull()
  })
})
