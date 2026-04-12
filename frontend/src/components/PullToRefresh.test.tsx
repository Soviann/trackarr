import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, waitFor } from '@testing-library/preact'
import { PullToRefresh } from './PullToRefresh'

// PointerEvent is not fully available in jsdom — provide a minimal constructor
function makePointerEvent(type: string, overrides: Partial<PointerEvent> = {}): PointerEvent {
  return new PointerEvent(type, {
    bubbles: true,
    cancelable: true,
    clientX: 0,
    clientY: 0,
    pointerId: 1,
    ...overrides,
  })
}

function dispatchOnWindow(event: PointerEvent) {
  window.dispatchEvent(event)
}

const THRESHOLD = 70

describe('PullToRefresh', () => {
  beforeEach(() => {
    // Ensure window.scrollY is 0 (simulate scrolled to top)
    Object.defineProperty(window, 'scrollY', { configurable: true, get: () => 0 })

    // jsdom doesn't provide navigator.vibrate — define it before spying
    if (!('vibrate' in navigator)) {
      Object.defineProperty(navigator, 'vibrate', {
        configurable: true,
        writable: true,
        value: () => true,
      })
    }
    vi.spyOn(navigator, 'vibrate').mockReturnValue(true)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('calls onRefresh when pulled past threshold and released', async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    const { container } = render(
      <PullToRefresh onRefresh={onRefresh} threshold={THRESHOLD}>
        <div>content</div>
      </PullToRefresh>
    )
    const wrapper = container.firstChild as HTMLElement

    // pointerdown on the container
    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientY: 0 }))
    })

    // Move past threshold
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientY: THRESHOLD + 20 }))
    })

    // Release
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientY: THRESHOLD + 20 }))
    })

    expect(onRefresh).toHaveBeenCalledOnce()
  })

  it('does NOT call onRefresh when released below threshold', async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    const { container } = render(
      <PullToRefresh onRefresh={onRefresh} threshold={THRESHOLD}>
        <div>content</div>
      </PullToRefresh>
    )
    const wrapper = container.firstChild as HTMLElement

    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientY: 0 }))
    })

    // Move just below threshold
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientY: THRESHOLD - 10 }))
    })

    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientY: THRESHOLD - 10 }))
    })

    expect(onRefresh).not.toHaveBeenCalled()
  })

  it('ignores a second pull while refresh is in progress', async () => {
    let resolveFirst: () => void
    const firstDone = new Promise<void>(res => { resolveFirst = res })
    const onRefresh = vi.fn().mockReturnValueOnce(firstDone).mockResolvedValue(undefined)

    const { container } = render(
      <PullToRefresh onRefresh={onRefresh} threshold={THRESHOLD}>
        <div>content</div>
      </PullToRefresh>
    )
    const wrapper = container.firstChild as HTMLElement

    // First gesture — triggers refresh
    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientY: 0 }))
    })
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientY: THRESHOLD + 20 }))
    })
    // Don't await — let refresh hang
    act(() => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientY: THRESHOLD + 20 }))
    })

    expect(onRefresh).toHaveBeenCalledTimes(1)

    // Second gesture during refresh — must be ignored
    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientY: 0, pointerId: 2 }))
    })
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientY: THRESHOLD + 20, pointerId: 2 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientY: THRESHOLD + 20, pointerId: 2 }))
    })

    expect(onRefresh).toHaveBeenCalledTimes(1)

    // Clean up: resolve the first refresh
    act(() => { resolveFirst!() })
    await waitFor(() => expect(onRefresh).toHaveBeenCalledTimes(1))
  })

  it('does nothing when disabled=true', async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    const { container } = render(
      <PullToRefresh onRefresh={onRefresh} threshold={THRESHOLD} disabled>
        <div>content</div>
      </PullToRefresh>
    )
    const wrapper = container.firstChild as HTMLElement

    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientY: 0 }))
    })
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientY: THRESHOLD + 20 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientY: THRESHOLD + 20 }))
    })

    expect(onRefresh).not.toHaveBeenCalled()
  })

  it('does not pull when window.scrollY > 0', async () => {
    // Override scrollY to non-zero
    Object.defineProperty(window, 'scrollY', { configurable: true, get: () => 50 })

    const onRefresh = vi.fn().mockResolvedValue(undefined)
    const { container } = render(
      <PullToRefresh onRefresh={onRefresh} threshold={THRESHOLD}>
        <div>content</div>
      </PullToRefresh>
    )
    const wrapper = container.firstChild as HTMLElement

    // pointerdown happens when scrollY > 0 — should be ignored
    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientY: 0 }))
    })
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientY: THRESHOLD + 20 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientY: THRESHOLD + 20 }))
    })

    expect(onRefresh).not.toHaveBeenCalled()
  })

  it('fires haptic once when crossing threshold going down', async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    // navigator.vibrate spy is already set up in beforeEach
    const vibrate = vi.mocked(navigator.vibrate)

    const { container } = render(
      <PullToRefresh onRefresh={onRefresh} threshold={THRESHOLD}>
        <div>content</div>
      </PullToRefresh>
    )
    const wrapper = container.firstChild as HTMLElement

    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientY: 0 }))
    })

    // Cross threshold
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientY: THRESHOLD + 10 }))
    })
    // Move further past threshold — haptic must NOT fire again
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientY: THRESHOLD + 30 }))
    })

    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientY: THRESHOLD + 30 }))
    })

    expect(vibrate).toHaveBeenCalledOnce()
  })
})
