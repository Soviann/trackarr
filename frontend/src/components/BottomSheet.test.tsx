import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { BottomSheet } from './BottomSheet'

function makePointerEvent(type: string, overrides: Partial<PointerEvent> = {}): PointerEvent {
  return new PointerEvent(type, { bubbles: true, cancelable: true, clientX: 0, clientY: 0, ...overrides })
}

describe('BottomSheet', () => {
  beforeEach(() => {
    // Reset body overflow before each test
    document.body.style.overflow = ''
    // Reset history pushState spy
    vi.restoreAllMocks()
  })

  afterEach(() => {
    cleanup()
    document.body.style.overflow = ''
  })

  // Test 1 — Renders children when open, nothing when closed
  it('renders children when open=true', () => {
    const { getByText } = render(
      <BottomSheet open={true} onClose={vi.fn()}>
        <p>Sheet content</p>
      </BottomSheet>,
    )
    expect(getByText('Sheet content')).toBeTruthy()
  })

  it('renders nothing when open=false', () => {
    const { container } = render(
      <BottomSheet open={false} onClose={vi.fn()}>
        <p>Sheet content</p>
      </BottomSheet>,
    )
    expect(container.firstChild).toBeNull()
  })

  // Test 2 — Overlay click calls onClose
  it('calls onClose when overlay is clicked', () => {
    const onClose = vi.fn()
    const { container } = render(
      <BottomSheet open={true} onClose={onClose}>
        <p>content</p>
      </BottomSheet>,
    )
    // The overlay is the root div
    const overlay = container.firstElementChild as HTMLElement
    fireEvent.click(overlay)
    expect(onClose).toHaveBeenCalledOnce()
  })

  // Test 3 — Drag down past 100px threshold calls onClose
  it('calls onClose when dragged past 100px threshold', () => {
    const onClose = vi.fn()
    const { container } = render(
      <BottomSheet open={true} onClose={onClose}>
        <p>content</p>
      </BottomSheet>,
    )
    const sheet = container.querySelector('[class*="sheet"]') as HTMLElement
    fireEvent(sheet, makePointerEvent('pointerdown', { clientY: 0 }))
    fireEvent(sheet, makePointerEvent('pointermove', { clientY: 150 }))
    fireEvent(sheet, makePointerEvent('pointerup'))
    expect(onClose).toHaveBeenCalledOnce()
  })

  // Test 4 — Drag below threshold snaps back, no onClose
  it('does not call onClose when drag is below threshold', () => {
    const onClose = vi.fn()
    const { container } = render(
      <BottomSheet open={true} onClose={onClose}>
        <p>content</p>
      </BottomSheet>,
    )
    const sheet = container.querySelector('[class*="sheet"]') as HTMLElement
    fireEvent(sheet, makePointerEvent('pointerdown', { clientY: 0 }))
    fireEvent(sheet, makePointerEvent('pointermove', { clientY: 50 }))
    fireEvent(sheet, makePointerEvent('pointerup'))
    expect(onClose).not.toHaveBeenCalled()
  })

  // Test 5 — Body overflow hidden when open, restored when closed
  it('sets body overflow hidden when open and restores when closed', () => {
    document.body.style.overflow = 'auto'
    const { rerender } = render(
      <BottomSheet open={true} onClose={vi.fn()}>
        <p>content</p>
      </BottomSheet>,
    )
    expect(document.body.style.overflow).toBe('hidden')

    rerender(
      <BottomSheet open={false} onClose={vi.fn()}>
        <p>content</p>
      </BottomSheet>,
    )
    expect(document.body.style.overflow).toBe('auto')
  })

  // Test 6 — history.pushState called when opening
  it('calls history.pushState when the sheet opens', () => {
    const pushState = vi.spyOn(history, 'pushState')
    render(
      <BottomSheet open={true} onClose={vi.fn()}>
        <p>content</p>
      </BottomSheet>,
    )
    expect(pushState).toHaveBeenCalledOnce()
    const [state] = pushState.mock.calls[0]
    expect((state as { token: string }).token).toMatch(/^bottomsheet-\d+$/)
  })

  // Test 7 — popstate event closes sheet
  it('calls onClose when a popstate event is dispatched', () => {
    const onClose = vi.fn()
    render(
      <BottomSheet open={true} onClose={onClose}>
        <p>content</p>
      </BottomSheet>,
    )
    window.dispatchEvent(new PopStateEvent('popstate', { state: null }))
    expect(onClose).toHaveBeenCalledOnce()
  })

  // Test 8 — role/aria attributes
  it('exposes role="dialog", aria-modal and aria-label', () => {
    const { container } = render(
      <BottomSheet open={true} onClose={vi.fn()} ariaLabel="Edit title">
        <p>content</p>
      </BottomSheet>,
    )
    const sheet = container.querySelector('[role="dialog"]') as HTMLElement
    expect(sheet).toBeTruthy()
    expect(sheet.getAttribute('aria-modal')).toBe('true')
    expect(sheet.getAttribute('aria-label')).toBe('Edit title')
  })

  it('falls back to aria-label="Dialog" when ariaLabel is not provided', () => {
    const { container } = render(
      <BottomSheet open={true} onClose={vi.fn()}>
        <p>content</p>
      </BottomSheet>,
    )
    const sheet = container.querySelector('[role="dialog"]') as HTMLElement
    expect(sheet.getAttribute('aria-label')).toBe('Dialog')
  })

  // Test 9 — Escape closes
  it('calls onClose when Escape is pressed', () => {
    const onClose = vi.fn()
    render(
      <BottomSheet open={true} onClose={onClose}>
        <button>OK</button>
      </BottomSheet>,
    )
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('does not react to Escape when closed', () => {
    const onClose = vi.fn()
    render(
      <BottomSheet open={false} onClose={onClose}>
        <button>OK</button>
      </BottomSheet>,
    )
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(onClose).not.toHaveBeenCalled()
  })

  // Test 10 — Initial focus on first focusable child
  it('moves focus to the first focusable child on open', () => {
    const { getByText } = render(
      <BottomSheet open={true} onClose={vi.fn()}>
        <button>First</button>
        <button>Second</button>
      </BottomSheet>,
    )
    expect(document.activeElement).toBe(getByText('First'))
  })

  // Test 11 — Focus restoration on close
  it('restores focus to the previously focused element on close', () => {
    const trigger = document.createElement('button')
    trigger.textContent = 'Open'
    document.body.appendChild(trigger)
    trigger.focus()
    expect(document.activeElement).toBe(trigger)

    const { rerender } = render(
      <BottomSheet open={true} onClose={vi.fn()}>
        <button>Inner</button>
      </BottomSheet>,
    )
    expect(document.activeElement).not.toBe(trigger)

    rerender(
      <BottomSheet open={false} onClose={vi.fn()}>
        <button>Inner</button>
      </BottomSheet>,
    )
    expect(document.activeElement).toBe(trigger)
    document.body.removeChild(trigger)
  })

  // Test 12 — Tab trap: forward from last wraps to first
  it('wraps focus to the first focusable when Tab is pressed on the last', () => {
    const { getByText } = render(
      <BottomSheet open={true} onClose={vi.fn()}>
        <button>One</button>
        <button>Two</button>
      </BottomSheet>,
    )
    const last = getByText('Two') as HTMLButtonElement
    last.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true }))
    expect(document.activeElement).toBe(getByText('One'))
  })

  // Test 13 — Tab trap: backward from first wraps to last
  it('wraps focus to the last focusable when Shift+Tab is pressed on the first', () => {
    const { getByText } = render(
      <BottomSheet open={true} onClose={vi.fn()}>
        <button>One</button>
        <button>Two</button>
      </BottomSheet>,
    )
    const first = getByText('One') as HTMLButtonElement
    first.focus()
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, bubbles: true }))
    expect(document.activeElement).toBe(getByText('Two'))
  })
})
