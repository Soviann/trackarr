import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook } from '@testing-library/preact'
import { useScrollRestoration, saveScroll, getSavedScroll, clearSavedScroll } from './useScrollRestoration'

describe('useScrollRestoration', () => {
  beforeEach(() => {
    clearSavedScroll('testKey')
    sessionStorage.clear()
    vi.restoreAllMocks()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('saves and retrieves scroll positions from memory and sessionStorage', () => {
    expect(getSavedScroll('testKey')).toBeUndefined()
    saveScroll('testKey', 350)
    expect(getSavedScroll('testKey')).toBe(350)
    expect(sessionStorage.getItem('scroll_testKey')).toBe('350')
  })

  it('does not restore scroll position when isReady is false', () => {
    saveScroll('testKey', 500)
    const scrollToSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})

    renderHook(({ isReady }) => useScrollRestoration('testKey', isReady), {
      initialProps: { isReady: false },
    })

    expect(scrollToSpy).not.toHaveBeenCalled()
  })

  it('restores scroll position when isReady is true', async () => {
    saveScroll('testKey', 500)
    const scrollToSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      cb(0)
      return 1
    })

    renderHook(({ isReady }) => useScrollRestoration('testKey', isReady), {
      initialProps: { isReady: true },
    })

    expect(scrollToSpy).toHaveBeenCalledWith(0, 500)
    rafSpy.mockRestore()
  })

  it('triggers scroll restoration when isReady transitions from false to true', () => {
    saveScroll('testKey', 400)
    const scrollToSpy = vi.spyOn(window, 'scrollTo').mockImplementation(() => {})
    const rafSpy = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      cb(0)
      return 1
    })

    const { rerender } = renderHook(({ isReady }) => useScrollRestoration('testKey', isReady), {
      initialProps: { isReady: false },
    })

    expect(scrollToSpy).not.toHaveBeenCalled()

    rerender({ isReady: true })

    expect(scrollToSpy).toHaveBeenCalledWith(0, 400)
    rafSpy.mockRestore()
  })
})
