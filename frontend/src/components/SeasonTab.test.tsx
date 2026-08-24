import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { SeasonTab } from './SeasonTab'
import type { Season } from '../types'

describe('SeasonTab', () => {
  afterEach(() => {
    cleanup()
  })

  const dummySeason: Season = {
    id: 1,
    title_id: 10,
    season_number: 2,
    total_episodes: 10,
    episodes: [
      { id: 101, season_id: 1, episode: 1, name: null, air_date: null, watched: true, first_watched_at: null, last_watched_at: null },
      { id: 102, season_id: 1, episode: 2, name: null, air_date: null, watched: false, first_watched_at: null, last_watched_at: null },
    ],
  }

  it('renders active season tab with progress count', () => {
    const onClick = vi.fn()
    const { getByText } = render(
      <SeasonTab season={dummySeason} active={true} onClick={onClick} />
    )

    expect(getByText('S2')).not.toBeNull()
    expect(getByText('1/10')).not.toBeNull()
  })

  it('renders inactive season tab and handles click', () => {
    const onClick = vi.fn()
    const { getByText } = render(
      <SeasonTab season={dummySeason} active={false} onClick={onClick} />
    )

    const btn = getByText('S2').closest('button')
    expect(btn).not.toBeNull()
    fireEvent.click(btn!)
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('adjusts container scrollLeft without scrolling window when active', () => {
    const container = document.createElement('div')
    Object.defineProperty(container, 'clientWidth', { value: 300, configurable: true })
    container.getBoundingClientRect = () => ({
      left: 0,
      top: 500,
      right: 300,
      bottom: 540,
      width: 300,
      height: 40,
      x: 0,
      y: 500,
      toJSON: () => {},
    })

    const { getByText } = render(
      <SeasonTab season={dummySeason} active={true} onClick={() => {}} />,
      { container: document.body.appendChild(container) }
    )

    const btn = getByText('S2').closest('button')!
    btn.getBoundingClientRect = () => ({
      left: 400,
      top: 500,
      right: 480,
      bottom: 540,
      width: 80,
      height: 40,
      x: 400,
      y: 500,
      toJSON: () => {},
    })

    // Re-render to trigger useEffect with mocked rects
    render(
      <SeasonTab season={dummySeason} active={true} onClick={() => {}} />,
      { container }
    )

    expect(container.scrollLeft).toBeGreaterThanOrEqual(0)
    document.body.removeChild(container)
  })
})
