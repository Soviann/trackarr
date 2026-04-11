import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/preact'
import { useLongPress } from './useLongPress'

function makePointerEvent(overrides: Partial<PointerEvent> = {}): PointerEvent {
  return { clientX: 0, clientY: 0, ...overrides } as PointerEvent
}

describe('useLongPress', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('calls onLongPress after threshold', () => {
    const onLongPress = vi.fn()
    const { result } = renderHook(() => useLongPress({ onLongPress, threshold: 500 }))
    const e = makePointerEvent()

    result.current.onPointerDown(e)
    vi.advanceTimersByTime(500)

    expect(onLongPress).toHaveBeenCalledOnce()
    expect(onLongPress).toHaveBeenCalledWith(e)
  })

  it('does not call onLongPress when released before threshold', () => {
    const onLongPress = vi.fn()
    const { result } = renderHook(() => useLongPress({ onLongPress, threshold: 500 }))
    const e = makePointerEvent()

    result.current.onPointerDown(e)
    vi.advanceTimersByTime(300)
    result.current.onPointerUp(e)
    vi.advanceTimersByTime(300)

    expect(onLongPress).not.toHaveBeenCalled()
  })

  it('does not call onLongPress when pointer moves beyond tolerance', () => {
    const onLongPress = vi.fn()
    const { result } = renderHook(() => useLongPress({ onLongPress, threshold: 500, moveTolerance: 10 }))

    result.current.onPointerDown(makePointerEvent({ clientX: 0, clientY: 0 } as PointerEvent))
    vi.advanceTimersByTime(300)
    result.current.onPointerMove(makePointerEvent({ clientX: 20, clientY: 0 } as PointerEvent))
    vi.advanceTimersByTime(300)

    expect(onLongPress).not.toHaveBeenCalled()
  })

  it('calls onClick when released before threshold and onClick is provided', () => {
    const onLongPress = vi.fn()
    const onClick = vi.fn()
    const { result } = renderHook(() => useLongPress({ onLongPress, onClick, threshold: 500 }))
    const e = makePointerEvent()

    result.current.onPointerDown(e)
    vi.advanceTimersByTime(200)
    result.current.onPointerUp(e)

    expect(onClick).toHaveBeenCalledOnce()
    expect(onLongPress).not.toHaveBeenCalled()
  })

  it('does not call onClick when long press has already fired', () => {
    const onLongPress = vi.fn()
    const onClick = vi.fn()
    const { result } = renderHook(() => useLongPress({ onLongPress, onClick, threshold: 500 }))
    const e = makePointerEvent()

    result.current.onPointerDown(e)
    vi.advanceTimersByTime(500)
    result.current.onPointerUp(e)

    expect(onLongPress).toHaveBeenCalledOnce()
    expect(onClick).not.toHaveBeenCalled()
  })

  it('cancels timer on pointerCancel', () => {
    const onLongPress = vi.fn()
    const { result } = renderHook(() => useLongPress({ onLongPress, threshold: 500 }))

    result.current.onPointerDown(makePointerEvent())
    vi.advanceTimersByTime(200)
    result.current.onPointerCancel()
    vi.advanceTimersByTime(400)

    expect(onLongPress).not.toHaveBeenCalled()
  })

  it('prevents native context menu via onContextMenu', () => {
    const { result } = renderHook(() => useLongPress({ onLongPress: vi.fn() }))
    const event = { preventDefault: vi.fn() } as unknown as Event

    result.current.onContextMenu(event)

    expect(event.preventDefault).toHaveBeenCalledOnce()
  })
})
