import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act } from '@testing-library/preact'
import { useIsDesktop } from './useIsDesktop'

describe('useIsDesktop', () => {
  let listeners: Array<(e: { matches: boolean }) => void> = []

  const mockMatchMedia = (initialMatches: boolean) => {
    listeners = []
    return vi.fn().mockImplementation((query: string) => ({
      matches: initialMatches,
      media: query,
      addEventListener: vi.fn((event: string, callback: (e: { matches: boolean }) => void) => {
        if (event === 'change') listeners.push(callback)
      }),
      removeEventListener: vi.fn((event: string, callback: (e: { matches: boolean }) => void) => {
        listeners = listeners.filter((l) => l !== callback)
      }),
    }))
  }

  beforeEach(() => {
    listeners = []
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('nominal: returns true when window matches desktop query', () => {
    window.matchMedia = mockMatchMedia(true)
    const { result } = renderHook(() => useIsDesktop())
    expect(result.current).toBe(true)
  })

  it('nominal: returns false when window does not match desktop query', () => {
    window.matchMedia = mockMatchMedia(false)
    const { result } = renderHook(() => useIsDesktop())
    expect(result.current).toBe(false)
  })

  it('updates state dynamically when media query change event fires', () => {
    window.matchMedia = mockMatchMedia(false)
    const { result } = renderHook(() => useIsDesktop())
    expect(result.current).toBe(false)

    act(() => {
      listeners.forEach((listener) => listener({ matches: true }))
    })
    expect(result.current).toBe(true)

    act(() => {
      listeners.forEach((listener) => listener({ matches: false }))
    })
    expect(result.current).toBe(false)
  })

  it('boundary: safely handles absence of matchMedia', () => {
    const original = window.matchMedia
    // @ts-expect-error simulating environment without matchMedia
    delete window.matchMedia
    const { result } = renderHook(() => useIsDesktop())
    expect(result.current).toBe(false)
    window.matchMedia = original
  })
})
