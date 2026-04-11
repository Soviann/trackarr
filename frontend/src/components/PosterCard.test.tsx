import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent } from '@testing-library/preact'
import { PosterCard } from './PosterCard'
import type { Title } from '../types'

// Stub zustand store used by PosterCard
vi.mock('../store', () => ({
  useTitleStore: (_sel: (s: unknown) => unknown) =>
    (_sel as (s: { sort: { field: string } }) => unknown)({ sort: { field: 'name' } }),
}))

const baseTitle: Title = {
  id: 42,
  name: 'Test Title',
  type: 'movie',
  status: 'watching',
  cover_url: null,
  last_watched_at: null,
  total_episodes: 0,
  watched_episodes: 0,
  year: 2024,
  source: 'tmdb',
  external_id: '123',
  created_at: '2024-01-01T00:00:00Z',
  updated_at: '2024-01-01T00:00:00Z',
}

function makePointerEvent(type: string, overrides: Partial<PointerEvent> = {}): PointerEvent {
  return new PointerEvent(type, { bubbles: true, cancelable: true, clientX: 0, clientY: 0, ...overrides })
}

describe('PosterCard', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders a link to the title page', () => {
    const { container } = render(<PosterCard title={baseTitle} />)
    const anchor = container.querySelector('a')
    expect(anchor).not.toBeNull()
    expect(anchor!.getAttribute('href')).toBe('/title/42')
  })

  it('calls onClick when clicked in selection mode (no long-press)', () => {
    const onClick = vi.fn()
    const { container } = render(<PosterCard title={baseTitle} onClick={onClick} />)
    const anchor = container.querySelector('a')!

    fireEvent.click(anchor)
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('does not call onClick when no onClick is provided (normal navigation)', () => {
    const onClick = vi.fn()
    const { container } = render(<PosterCard title={baseTitle} />)
    const anchor = container.querySelector('a')!

    fireEvent.click(anchor)
    expect(onClick).not.toHaveBeenCalled()
  })

  it('calls onLongPress after 500ms hold', () => {
    const onLongPress = vi.fn()
    const { container } = render(<PosterCard title={baseTitle} onLongPress={onLongPress} />)
    const anchor = container.querySelector('a')!

    fireEvent(anchor, makePointerEvent('pointerdown'))
    vi.advanceTimersByTime(500)

    expect(onLongPress).toHaveBeenCalledOnce()
  })

  it('does not call onLongPress when released before threshold', () => {
    const onLongPress = vi.fn()
    const { container } = render(<PosterCard title={baseTitle} onLongPress={onLongPress} />)
    const anchor = container.querySelector('a')!

    fireEvent(anchor, makePointerEvent('pointerdown'))
    vi.advanceTimersByTime(300)
    fireEvent(anchor, makePointerEvent('pointerup'))

    expect(onLongPress).not.toHaveBeenCalled()
  })

  it('suppresses click navigation after a long-press fires', () => {
    const onLongPress = vi.fn()
    const onClick = vi.fn()
    const { container } = render(<PosterCard title={baseTitle} onLongPress={onLongPress} onClick={onClick} />)
    const anchor = container.querySelector('a')!

    fireEvent(anchor, makePointerEvent('pointerdown'))
    vi.advanceTimersByTime(500)

    // Simulate the click that follows pointer-up after a long press
    const clickEvent = new MouseEvent('click', { bubbles: true, cancelable: true })
    anchor.dispatchEvent(clickEvent)

    expect(onLongPress).toHaveBeenCalledOnce()
    expect(onClick).not.toHaveBeenCalled()
    expect(clickEvent.defaultPrevented).toBe(true)
  })

  it('does not suppress click on the next regular click after a long-press click was consumed', () => {
    const onLongPress = vi.fn()
    const onClick = vi.fn()
    const { container } = render(<PosterCard title={baseTitle} onLongPress={onLongPress} onClick={onClick} />)
    const anchor = container.querySelector('a')!

    // Trigger long press and consume the following click
    fireEvent(anchor, makePointerEvent('pointerdown'))
    vi.advanceTimersByTime(500)
    fireEvent.click(anchor) // consumed by justFiredRef

    // Second regular click (no long press) should call onClick normally
    fireEvent.click(anchor)
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('applies no-touch-callout class to prevent text selection callout', () => {
    const { container } = render(<PosterCard title={baseTitle} onLongPress={vi.fn()} />)
    const anchor = container.querySelector('a')!
    expect(anchor.classList.contains('no-touch-callout')).toBe(true)
  })

  it('resets the click-suppression flag on a new pointerdown even if no click followed the long-press', () => {
    const onLongPress = vi.fn()
    const onClick = vi.fn()

    // First render: not in selection mode — no onClick
    const { container, rerender } = render(<PosterCard title={baseTitle} onLongPress={onLongPress} />)
    const anchor = container.querySelector('a')!

    // Long-press fires
    fireEvent(anchor, makePointerEvent('pointerdown'))
    vi.advanceTimersByTime(500)
    expect(onLongPress).toHaveBeenCalledTimes(1)

    // User slides finger off the card — no click is dispatched
    fireEvent(anchor, makePointerEvent('pointerup'))

    // Re-render with onClick (selection mode now active)
    rerender(<PosterCard title={baseTitle} onLongPress={onLongPress} onClick={onClick} />)

    // New short tap — pointerdown resets the flag, so click must reach onClick
    fireEvent(anchor, makePointerEvent('pointerdown'))
    fireEvent(anchor, makePointerEvent('pointerup'))
    fireEvent.click(anchor)

    expect(onClick).toHaveBeenCalledTimes(1)
  })
})
