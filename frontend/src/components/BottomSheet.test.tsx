import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent } from '@testing-library/preact'
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
})
