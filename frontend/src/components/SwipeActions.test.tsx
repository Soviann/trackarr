import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, act, within } from '@testing-library/preact'
import { SwipeActions } from './SwipeActions'
import type { SwipeAction } from './SwipeActions'

// PointerEvent minimal constructor for jsdom
function makePointerEvent(type: string, overrides: Partial<PointerEventInit> = {}): PointerEvent {
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

// jsdom doesn't lay out elements; fake offsetWidth so threshold math works
function mockContainerWidth(el: HTMLElement, width = 400) {
  Object.defineProperty(el, 'offsetWidth', { configurable: true, get: () => width })
  Object.defineProperty(el, 'offsetHeight', { configurable: true, get: () => 80 })
}
// Fake scrollWidth on actions panel
function mockActionsWidth(container: HTMLElement | Element, width = 160) {
  const actionsEl = container.querySelector('[aria-hidden="true"]')
  if (actionsEl) {
    Object.defineProperty(actionsEl, 'scrollWidth', { configurable: true, get: () => width })
  }
}

const ACTIONS_WIDTH = 160
const CONTAINER_WIDTH = 400
// 40% threshold = 160px — exactly equals ACTIONS_WIDTH so we use 170px to pass threshold
const SWIPE_PAST_THRESHOLD = 170

describe('SwipeActions', () => {
  let actions: SwipeAction[]
  let onConfirm: ReturnType<typeof vi.fn>
  let onFix: ReturnType<typeof vi.fn>

  beforeEach(() => {
    const confirmFn = vi.fn().mockResolvedValue(undefined)
    const fixFn = vi.fn()
    onConfirm = confirmFn
    onFix = fixFn
    actions = [
      { icon: '✓', color: 'green', label: 'Confirm', onAction: () => { confirmFn() } },
      { icon: '✎', color: 'orange', label: 'Fix', onAction: () => { fixFn() } },
    ]

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
    document.dispatchEvent(new CustomEvent('swipe-actions-close', { detail: { id: '__cleanup__' } }))
  })

  it('swipe left past threshold reveals actions and auto-executes primary action on far drag', async () => {
    const { container } = render(
      <SwipeActions actions={actions}>
        <div>card content</div>
      </SwipeActions>
    )
    const wrapper = container.firstChild as HTMLElement
    mockContainerWidth(wrapper)
    mockActionsWidth(container)

    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientX: 300, clientY: 0 }))
    })
    // Drag well past threshold (> 1.1 × ACTIONS_WIDTH = 176px)
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientX: 300 - 200, clientY: 0 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientX: 300 - 200, clientY: 0 }))
    })

    // Primary action should have been called (after animation timeout)
    await vi.waitFor(() => {
      expect(onConfirm).toHaveBeenCalledOnce()
    }, { timeout: 500 })
  })

  it('swipe left partial reveals action buttons', async () => {
    const { container } = render(
      <SwipeActions actions={actions}>
        <div>card content</div>
      </SwipeActions>
    )
    const wrapper = container.firstChild as HTMLElement
    mockContainerWidth(wrapper, CONTAINER_WIDTH)
    mockActionsWidth(container, ACTIONS_WIDTH)

    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientX: 300, clientY: 0 }))
    })
    // Drag past 15px dead zone but not far enough for auto-execute
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientX: 300 - SWIPE_PAST_THRESHOLD, clientY: 0 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientX: 300 - SWIPE_PAST_THRESHOLD, clientY: 0 }))
    })

    // Action buttons should be in DOM (scoped to this container, hidden:true for aria-hidden panel)
    const q = within(container as HTMLElement)
    const confirmBtn = q.getByRole('button', { name: 'Confirm', hidden: true })
    const fixBtn = q.getByRole('button', { name: 'Fix', hidden: true })
    expect(confirmBtn).toBeTruthy()
    expect(fixBtn).toBeTruthy()
    // onConfirm not called yet — just revealed
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('tapping a specific action calls that action', async () => {
    const { container } = render(
      <SwipeActions actions={actions}>
        <div>card content</div>
      </SwipeActions>
    )
    const wrapper = container.firstChild as HTMLElement
    mockContainerWidth(wrapper, CONTAINER_WIDTH)
    mockActionsWidth(container, ACTIONS_WIDTH)

    // Swipe to reveal
    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientX: 300, clientY: 0 }))
    })
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientX: 300 - SWIPE_PAST_THRESHOLD, clientY: 0 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientX: 300 - SWIPE_PAST_THRESHOLD, clientY: 0 }))
    })

    // Tap the second action ("Fix") — scoped to this container
    const fixBtn = within(container as HTMLElement).getByRole('button', { name: 'Fix', hidden: true })
    await act(async () => {
      fixBtn.click()
    })

    expect(onFix).toHaveBeenCalledOnce()
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('vertical swipe does not move the content horizontally', async () => {
    const { container } = render(
      <SwipeActions actions={actions}>
        <div>card content</div>
      </SwipeActions>
    )
    const wrapper = container.firstChild as HTMLElement
    mockContainerWidth(wrapper, CONTAINER_WIDTH)
    mockActionsWidth(container, ACTIONS_WIDTH)

    // Find the content wrapper — it's the second child of the container (after the actions panel)
    const contentEl = wrapper.children[1] as HTMLElement | null

    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientX: 0, clientY: 0 }))
    })
    // Primarily vertical movement (dy > dx)
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientX: -5, clientY: 60 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientX: -5, clientY: 60 }))
    })

    // Content should remain at translateX(0) — vertical gesture was ignored
    expect(contentEl?.style.transform).toBe('translateX(0px)')
    expect(onConfirm).not.toHaveBeenCalled()
    expect(onFix).not.toHaveBeenCalled()
  })

  it('disabled=true prevents any swipe', async () => {
    const { container } = render(
      <SwipeActions actions={actions} disabled>
        <div>card content</div>
      </SwipeActions>
    )
    const wrapper = container.firstChild as HTMLElement
    mockContainerWidth(wrapper, CONTAINER_WIDTH)
    mockActionsWidth(container, ACTIONS_WIDTH)

    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientX: 300, clientY: 0 }))
    })
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientX: 300 - 200, clientY: 0 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientX: 300 - 200, clientY: 0 }))
    })

    expect(onConfirm).not.toHaveBeenCalled()
    expect(onFix).not.toHaveBeenCalled()
  })

  it('haptic fires once when crossing the reveal threshold', async () => {
    const vibrate = vi.mocked(navigator.vibrate)
    const { container } = render(
      <SwipeActions actions={actions}>
        <div>card content</div>
      </SwipeActions>
    )
    const wrapper = container.firstChild as HTMLElement
    mockContainerWidth(wrapper, CONTAINER_WIDTH)
    mockActionsWidth(container, ACTIONS_WIDTH)

    act(() => {
      wrapper.dispatchEvent(makePointerEvent('pointerdown', { clientX: 300, clientY: 0 }))
    })
    // Cross threshold
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientX: 300 - SWIPE_PAST_THRESHOLD, clientY: 0 }))
    })
    // Move further — haptic must NOT re-fire
    act(() => {
      dispatchOnWindow(makePointerEvent('pointermove', { clientX: 300 - SWIPE_PAST_THRESHOLD - 30, clientY: 0 }))
    })
    await act(async () => {
      dispatchOnWindow(makePointerEvent('pointerup', { clientX: 300 - SWIPE_PAST_THRESHOLD - 30, clientY: 0 }))
    })

    expect(vibrate).toHaveBeenCalledOnce()
  })
})
