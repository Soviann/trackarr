import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, waitFor } from '@testing-library/preact'
import { PullToRefresh } from './PullToRefresh'

function touchStart(el: HTMLElement, clientY: number) {
  el.dispatchEvent(new TouchEvent('touchstart', {
    bubbles: true,
    cancelable: true,
    touches: [{ clientX: 0, clientY, identifier: 0, target: el } as Touch],
  }))
}

function touchMove(el: HTMLElement, clientY: number) {
  el.dispatchEvent(new TouchEvent('touchmove', {
    bubbles: true,
    cancelable: true,
    touches: [{ clientX: 0, clientY, identifier: 0, target: el } as Touch],
  }))
}

function touchEnd(el: HTMLElement) {
  el.dispatchEvent(new TouchEvent('touchend', {
    bubbles: true,
    cancelable: true,
    touches: [],
  }))
}

const THRESHOLD = 70

describe('PullToRefresh', () => {
  beforeEach(() => {
    Object.defineProperty(window, 'scrollY', { configurable: true, get: () => 0 })

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

    act(() => { touchStart(wrapper, 0) })
    act(() => { touchMove(wrapper, THRESHOLD + 20) })
    await act(async () => { touchEnd(wrapper) })

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

    act(() => { touchStart(wrapper, 0) })
    act(() => { touchMove(wrapper, THRESHOLD - 10) })
    await act(async () => { touchEnd(wrapper) })

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
    act(() => { touchStart(wrapper, 0) })
    act(() => { touchMove(wrapper, THRESHOLD + 20) })
    act(() => { touchEnd(wrapper) })

    expect(onRefresh).toHaveBeenCalledTimes(1)

    // Second gesture during refresh — must be ignored
    act(() => { touchStart(wrapper, 0) })
    act(() => { touchMove(wrapper, THRESHOLD + 20) })
    await act(async () => { touchEnd(wrapper) })

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

    act(() => { touchStart(wrapper, 0) })
    act(() => { touchMove(wrapper, THRESHOLD + 20) })
    await act(async () => { touchEnd(wrapper) })

    expect(onRefresh).not.toHaveBeenCalled()
  })

  it('does not pull when window.scrollY > 0', async () => {
    Object.defineProperty(window, 'scrollY', { configurable: true, get: () => 50 })

    const onRefresh = vi.fn().mockResolvedValue(undefined)
    const { container } = render(
      <PullToRefresh onRefresh={onRefresh} threshold={THRESHOLD}>
        <div>content</div>
      </PullToRefresh>
    )
    const wrapper = container.firstChild as HTMLElement

    act(() => { touchStart(wrapper, 0) })
    act(() => { touchMove(wrapper, THRESHOLD + 20) })
    await act(async () => { touchEnd(wrapper) })

    expect(onRefresh).not.toHaveBeenCalled()
  })

  it('fires haptic once when crossing threshold going down', async () => {
    const onRefresh = vi.fn().mockResolvedValue(undefined)
    const vibrate = vi.mocked(navigator.vibrate)

    const { container } = render(
      <PullToRefresh onRefresh={onRefresh} threshold={THRESHOLD}>
        <div>content</div>
      </PullToRefresh>
    )
    const wrapper = container.firstChild as HTMLElement

    act(() => { touchStart(wrapper, 0) })
    act(() => { touchMove(wrapper, THRESHOLD + 10) })
    act(() => { touchMove(wrapper, THRESHOLD + 30) })
    await act(async () => { touchEnd(wrapper) })

    expect(vibrate).toHaveBeenCalledOnce()
  })
})
