import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { PosterTile, type PosterTileItem } from './PosterTile'

describe('PosterTile', () => {
  afterEach(() => {
    cleanup()
  })
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

  it('renders streaming provider badges when watch_providers are given', () => {
    const item: PosterTileItem = {
      id: 3,
      type: 'series',
      cover_url: null,
      name: 'Dan Da Dan',
      sublabel: 'S01E01',
      watch_providers: [
        { id: 8, name: 'Netflix' },
        { id: 283, name: 'Crunchyroll' },
      ],
    }

    const { getByText } = render(<PosterTile item={item} />)
    expect(getByText('netflix')).not.toBeNull()
    expect(getByText('crunchyroll')).not.toBeNull()
  })

  it('renders quick mark button when next_episode and onQuickMark are provided', () => {
    const onQuickMark = vi.fn()
    const item: PosterTileItem = {
      id: 4,
      type: 'series',
      cover_url: null,
      name: 'Severance',
      sublabel: 'S02E01',
      progressRatio: 0.5,
      next_episode: {
        id: 101,
        season_id: 10,
        episode: 1,
        season_number: 2,
      },
      onQuickMark,
    }

    const { getByLabelText, getByText } = render(<PosterTile item={item} />)
    const btn = getByLabelText('Mark S2 E1 as watched')
    expect(btn).not.toBeNull()
    expect(getByText('+1')).not.toBeNull()

    fireEvent.click(btn)
    expect(onQuickMark).toHaveBeenCalledOnce()
  })

  it('shows loading state when isMarking is true', () => {
    const onQuickMark = vi.fn()
    const item: PosterTileItem = {
      id: 5,
      type: 'series',
      cover_url: null,
      name: 'Severance',
      sublabel: 'S02E01',
      progressRatio: 0.5,
      next_episode: {
        id: 101,
        season_id: 10,
        episode: 1,
        season_number: 2,
      },
      onQuickMark,
      isMarking: true,
    }

    const { getByLabelText } = render(<PosterTile item={item} />)
    const btn = getByLabelText('Mark S2 E1 as watched') as HTMLButtonElement
    expect(btn.disabled).toBe(true)

    fireEvent.click(btn)
    expect(onQuickMark).not.toHaveBeenCalled()
  })
})
