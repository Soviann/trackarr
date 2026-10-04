import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/preact'
import { useSwipeDownToClose } from './useSwipeDownToClose'

function makeTouchEvent(type: string, clientY: number, cancelable = true): TouchEvent {
  const touch = { clientY } as Touch
  const event = new Event(type, { bubbles: true, cancelable }) as unknown as TouchEvent
  Object.defineProperty(event, 'touches', {
    value: [touch],
  })
  return event
}

describe('useSwipeDownToClose', () => {
  let element: HTMLDivElement

  beforeEach(() => {
    element = document.createElement('div')
    document.body.appendChild(element)
  })

  afterEach(() => {
    document.body.removeChild(element)
  })

  it('initializes with dragY=0 and undefined style', () => {
    const onClose = vi.fn()
    const { result } = renderHook(() => useSwipeDownToClose({ open: true, onClose }))
    expect(result.current.dragY).toBe(0)
    expect(result.current.style).toBeUndefined()
  })

  it('tracks dragY and calls onClose when dragged beyond threshold', () => {
    const onClose = vi.fn()
    const { result } = renderHook(() => useSwipeDownToClose({ open: true, onClose, threshold: 80 }))
    Object.defineProperty(result.current.ref, 'current', { value: element, writable: true })

    // Force re-attach listener with the mocked element ref
    const { rerender } = renderHook(() => {
      const hook = useSwipeDownToClose({ open: true, onClose, threshold: 80 })
      hook.ref.current = element
      return hook
    })

    act(() => {
      element.dispatchEvent(makeTouchEvent('touchstart', 100))
      element.dispatchEvent(makeTouchEvent('touchmove', 200))
    })

    expect(onClose).not.toHaveBeenCalled()

    act(() => {
      element.dispatchEvent(makeTouchEvent('touchend', 200))
    })

    expect(onClose).toHaveBeenCalledOnce()
  })

  it('does not call onClose when released below threshold', () => {
    const onClose = vi.fn()
    renderHook(() => {
      const hook = useSwipeDownToClose({ open: true, onClose, threshold: 100 })
      hook.ref.current = element
      return hook
    })

    act(() => {
      element.dispatchEvent(makeTouchEvent('touchstart', 100))
      element.dispatchEvent(makeTouchEvent('touchmove', 150)) // delta 50 < 100
      element.dispatchEvent(makeTouchEvent('touchend', 150))
    })

    expect(onClose).not.toHaveBeenCalled()
  })

  it('ignores touches when open is false', () => {
    const onClose = vi.fn()
    renderHook(() => {
      const hook = useSwipeDownToClose({ open: false, onClose, threshold: 50 })
      hook.ref.current = element
      return hook
    })

    act(() => {
      element.dispatchEvent(makeTouchEvent('touchstart', 100))
      element.dispatchEvent(makeTouchEvent('touchmove', 200))
      element.dispatchEvent(makeTouchEvent('touchend', 200))
    })

    expect(onClose).not.toHaveBeenCalled()
  })

  it('ignores touches when shouldIgnore returns true', () => {
    const onClose = vi.fn()
    const child = document.createElement('div')
    element.appendChild(child)

    renderHook(() => {
      const hook = useSwipeDownToClose({
        open: true,
        onClose,
        shouldIgnore: (target) => target === child,
      })
      hook.ref.current = element
      return hook
    })

    act(() => {
      child.dispatchEvent(makeTouchEvent('touchstart', 100))
      child.dispatchEvent(makeTouchEvent('touchmove', 250))
      child.dispatchEvent(makeTouchEvent('touchend', 250))
    })

    expect(onClose).not.toHaveBeenCalled()
  })

  it('does not close when touch starts on an internally scrolled element (scrollTop > 0)', () => {
    const onClose = vi.fn()
    const scrollableChild = document.createElement('div')
    Object.defineProperty(scrollableChild, 'scrollTop', { value: 50, writable: true })
    element.appendChild(scrollableChild)

    renderHook(() => {
      const hook = useSwipeDownToClose({ open: true, onClose, threshold: 50 })
      hook.ref.current = element
      return hook
    })

    act(() => {
      scrollableChild.dispatchEvent(makeTouchEvent('touchstart', 100))
      scrollableChild.dispatchEvent(makeTouchEvent('touchmove', 250))
      scrollableChild.dispatchEvent(makeTouchEvent('touchend', 250))
    })

    expect(onClose).not.toHaveBeenCalled()
  })

  it('calls stopPropagation on downward touchmove to isolate gesture from PullToRefresh', () => {
    const onClose = vi.fn()
    renderHook(() => {
      const hook = useSwipeDownToClose({ open: true, onClose, threshold: 50 })
      hook.ref.current = element
      return hook
    })

    const moveEvent = makeTouchEvent('touchmove', 200)
    const stopSpy = vi.spyOn(moveEvent, 'stopPropagation')

    act(() => {
      element.dispatchEvent(makeTouchEvent('touchstart', 100))
      element.dispatchEvent(moveEvent)
    })

    expect(stopSpy).toHaveBeenCalled()
  })
})
