import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, fireEvent, cleanup } from '@testing-library/preact'
import { EpisodeRow } from './EpisodeRow'
import type { Episode } from '../types'
import * as hapticModule from '../utils/haptic'

vi.spyOn(hapticModule, 'haptic').mockImplementation(() => {})

const mockEpisode: Episode = {
  id: 42,
  season_id: 1,
  episode: 5,
  name: 'The Great Expedition',
  watched: false,
  air_date: '2024-02-20',
  first_watched_at: null,
  last_watched_at: null,
}

describe('EpisodeRow', () => {
  afterEach(() => {
    cleanup()
    vi.clearAllMocks()
  })

  it('renders episode number, name, air date and unwatched checkbox', () => {
    const onToggle = vi.fn()
    render(<EpisodeRow episode={mockEpisode} onToggle={onToggle} />)

    expect(screen.getByText('E5')).toBeTruthy()
    expect(screen.getByText('— The Great Expedition')).toBeTruthy()
    expect(screen.getByText('2024-02-20')).toBeTruthy()
  })

  it('triggers onToggle when toggle button is clicked', () => {
    const onToggle = vi.fn()
    render(<EpisodeRow episode={mockEpisode} onToggle={onToggle} />)

    const btn = screen.getByRole('button')
    fireEvent.click(btn)

    expect(onToggle).toHaveBeenCalledWith(42)
  })

  it('triggers onToggle and haptic when touch swipe exceeds threshold', () => {
    const onToggle = vi.fn()
    const { container } = render(<EpisodeRow episode={mockEpisode} onToggle={onToggle} />)
    const swipeContainer = container.firstChild as HTMLElement

    // Pointer down with touch
    fireEvent.pointerDown(swipeContainer, {
      pointerId: 1,
      pointerType: 'touch',
      clientX: 50,
      clientY: 100,
    })

    // Pointer move past 64px threshold (e.g. delta 80px)
    fireEvent.pointerMove(swipeContainer, {
      pointerId: 1,
      pointerType: 'touch',
      clientX: 140,
      clientY: 100,
    })

    expect(hapticModule.haptic).toHaveBeenCalled()

    // Pointer up to release
    fireEvent.pointerUp(swipeContainer, {
      pointerId: 1,
      pointerType: 'touch',
    })

    expect(onToggle).toHaveBeenCalledWith(42)
  })

  it('does not trigger onToggle if swipe distance is below threshold', () => {
    const onToggle = vi.fn()
    const { container } = render(<EpisodeRow episode={mockEpisode} onToggle={onToggle} />)
    const swipeContainer = container.firstChild as HTMLElement

    // Pointer down with touch
    fireEvent.pointerDown(swipeContainer, {
      pointerId: 1,
      pointerType: 'touch',
      clientX: 50,
      clientY: 100,
    })

    // Pointer move only 20px
    fireEvent.pointerMove(swipeContainer, {
      pointerId: 1,
      pointerType: 'touch',
      clientX: 70,
      clientY: 100,
    })

    expect(hapticModule.haptic).not.toHaveBeenCalled()

    // Pointer up to release
    fireEvent.pointerUp(swipeContainer, {
      pointerId: 1,
      pointerType: 'touch',
    })

    expect(onToggle).not.toHaveBeenCalled()
  })

  it('ignores mouse dragging so desktop click/selection is preserved', () => {
    const onToggle = vi.fn()
    const { container } = render(<EpisodeRow episode={mockEpisode} onToggle={onToggle} />)
    const swipeContainer = container.firstChild as HTMLElement

    // Pointer down with mouse
    fireEvent.pointerDown(swipeContainer, {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: 50,
      clientY: 100,
    })

    // Pointer move
    fireEvent.pointerMove(swipeContainer, {
      pointerId: 1,
      pointerType: 'mouse',
      clientX: 150,
      clientY: 100,
    })

    fireEvent.pointerUp(swipeContainer, {
      pointerId: 1,
      pointerType: 'mouse',
    })

    expect(hapticModule.haptic).not.toHaveBeenCalled()
    expect(onToggle).not.toHaveBeenCalled()
  })
})
