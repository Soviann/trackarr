import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, fireEvent, act, cleanup } from '@testing-library/preact'
import { UndoProvider, useUndo } from '../context/UndoContext'
import { UndoSnackbar } from './UndoSnackbar'

function TestTrigger({
  onUndo,
  onExpire,
  durationMs = 5000,
}: {
  onUndo: () => void | Promise<void>
  onExpire?: () => void | Promise<void>
  durationMs?: number
}) {
  const { showUndo } = useUndo()
  return (
    <button
      type="button"
      onClick={() =>
        showUndo({
          message: 'Marked S01E01 as watched',
          onUndo,
          onExpire,
          durationMs,
        })
      }
    >
      Trigger Undo
    </button>
  )
}

describe('UndoSnackbar', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    cleanup()
    vi.restoreAllMocks()
  })

  it('renders nothing when there is no active undo action', () => {
    const { container } = render(
      <UndoProvider>
        <UndoSnackbar />
      </UndoProvider>
    )
    expect(container.querySelector('[role="status"]')).toBeNull()
  })

  it('renders message, perimeter seconds and undo button when triggered', () => {
    const onUndo = vi.fn()
    const { getByText, container } = render(
      <UndoProvider>
        <TestTrigger onUndo={onUndo} />
        <UndoSnackbar />
      </UndoProvider>
    )

    act(() => {
      fireEvent.click(getByText('Trigger Undo'))
    })

    expect(getByText('Marked S01E01 as watched')).toBeTruthy()
    expect(getByText('Undo')).toBeTruthy()
    expect(container.querySelector('svg circle')).toBeTruthy()
  })

  it('executes onUndo and dismisses when Undo button is clicked', async () => {
    const onUndo = vi.fn()
    const { getByText, queryByText } = render(
      <UndoProvider>
        <TestTrigger onUndo={onUndo} />
        <UndoSnackbar />
      </UndoProvider>
    )

    act(() => {
      fireEvent.click(getByText('Trigger Undo'))
    })

    expect(getByText('Marked S01E01 as watched')).toBeTruthy()

    await act(async () => {
      fireEvent.click(getByText('Undo'))
    })

    expect(onUndo).toHaveBeenCalledOnce()
    expect(queryByText('Marked S01E01 as watched')).toBeNull()
  })

  it('dismisses when close button is clicked without calling onUndo', () => {
    const onUndo = vi.fn()
    const onExpire = vi.fn()
    const { getByText, getByRole, queryByText } = render(
      <UndoProvider>
        <TestTrigger onUndo={onUndo} onExpire={onExpire} />
        <UndoSnackbar />
      </UndoProvider>
    )

    act(() => {
      fireEvent.click(getByText('Trigger Undo'))
    })

    const closeBtn = getByRole('button', { name: /dismiss|ignorer/i })
    act(() => {
      fireEvent.click(closeBtn)
    })

    expect(onUndo).not.toHaveBeenCalled()
    expect(onExpire).toHaveBeenCalledOnce()
    expect(queryByText('Marked S01E01 as watched')).toBeNull()
  })

  it('auto-dismisses and calls onExpire when duration expires', () => {
    const onUndo = vi.fn()
    const onExpire = vi.fn()
    const { getByText, queryByText } = render(
      <UndoProvider>
        <TestTrigger onUndo={onUndo} onExpire={onExpire} durationMs={3000} />
        <UndoSnackbar />
      </UndoProvider>
    )

    act(() => {
      fireEvent.click(getByText('Trigger Undo'))
    })

    expect(getByText('Marked S01E01 as watched')).toBeTruthy()

    act(() => {
      vi.advanceTimersByTime(3500)
    })

    expect(onUndo).not.toHaveBeenCalled()
    expect(onExpire).toHaveBeenCalledOnce()
    expect(queryByText('Marked S01E01 as watched')).toBeNull()
  })
})
