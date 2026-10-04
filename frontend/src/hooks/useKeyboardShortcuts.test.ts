import { describe, it, expect, vi } from 'vitest'
import { renderHook } from '@testing-library/preact'
import { useKeyboardShortcuts } from './useKeyboardShortcuts'

describe('useKeyboardShortcuts', () => {
  it('triggers onSearch when Cmd+K or Ctrl+K is pressed', () => {
    const onSearch = vi.fn()
    renderHook(() => useKeyboardShortcuts({ onSearch }))

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    expect(onSearch).toHaveBeenCalledTimes(1)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'K', ctrlKey: true }))
    expect(onSearch).toHaveBeenCalledTimes(2)
  })

  it('triggers onSearch when "/" is pressed outside input fields', () => {
    const onSearch = vi.fn()
    renderHook(() => useKeyboardShortcuts({ onSearch }))

    window.dispatchEvent(new KeyboardEvent('keydown', { key: '/' }))
    expect(onSearch).toHaveBeenCalledTimes(1)
  })

  it('does NOT trigger onSearch when "/" is pressed with meta, ctrl, or alt keys', () => {
    const onSearch = vi.fn()
    renderHook(() => useKeyboardShortcuts({ onSearch }))

    window.dispatchEvent(new KeyboardEvent('keydown', { key: '/', metaKey: true }))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: '/', ctrlKey: true }))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: '/', altKey: true }))
    expect(onSearch).not.toHaveBeenCalled()
  })

  it('does NOT trigger onSearch when "/" is pressed inside an input or textarea', () => {
    const onSearch = vi.fn()
    renderHook(() => useKeyboardShortcuts({ onSearch }))

    const input = document.createElement('input')
    document.body.appendChild(input)

    const event = new KeyboardEvent('keydown', { key: '/', bubbles: true })
    input.dispatchEvent(event)

    expect(onSearch).not.toHaveBeenCalled()
    document.body.removeChild(input)
  })

  it('triggers onEscape when Escape key is pressed', () => {
    const onSearch = vi.fn()
    const onEscape = vi.fn()
    renderHook(() => useKeyboardShortcuts({ onSearch, onEscape }))

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(onEscape).toHaveBeenCalledTimes(1)
  })

  it('does not trigger callbacks when enabled is false', () => {
    const onSearch = vi.fn()
    const onEscape = vi.fn()
    renderHook(() => useKeyboardShortcuts({ onSearch, onEscape, enabled: false }))

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'k', metaKey: true }))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: '/' }))
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))

    expect(onSearch).not.toHaveBeenCalled()
    expect(onEscape).not.toHaveBeenCalled()
  })
})
