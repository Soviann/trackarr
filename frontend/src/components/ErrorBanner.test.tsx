import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, fireEvent, cleanup } from '@testing-library/preact'
import { ErrorBanner } from './ErrorBanner'

afterEach(() => cleanup())

describe('ErrorBanner', () => {
  it('renders message', () => {
    const { getByText } = render(<ErrorBanner message="Something failed" />)
    expect(getByText('Something failed')).toBeTruthy()
  })

  it('renders nothing when message is empty', () => {
    const { container } = render(<ErrorBanner message="" />)
    expect(container.innerHTML).toBe('')
  })

  it('shows retry button when onRetry provided', () => {
    const onRetry = vi.fn()
    const { getByText } = render(<ErrorBanner message="Error" onRetry={onRetry} />)
    fireEvent.click(getByText('Retry'))
    expect(onRetry).toHaveBeenCalledOnce()
  })

  it('hides retry button when onRetry not provided', () => {
    const { container } = render(<ErrorBanner message="Error" />)
    expect(container.querySelectorAll('button').length).toBe(0)
  })

  it('shows dismiss button when onDismiss provided', () => {
    const onDismiss = vi.fn()
    const { getByText } = render(<ErrorBanner message="Error" onDismiss={onDismiss} />)
    fireEvent.click(getByText('✕'))
    expect(onDismiss).toHaveBeenCalledOnce()
  })

  it('shows both buttons when both callbacks provided', () => {
    const onRetry = vi.fn()
    const onDismiss = vi.fn()
    const { container } = render(<ErrorBanner message="Error" onRetry={onRetry} onDismiss={onDismiss} />)
    expect(container.querySelectorAll('button').length).toBe(2)
  })
})
