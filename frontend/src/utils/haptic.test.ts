import { describe, it, expect, vi, beforeEach } from 'vitest'
import { haptic, HAPTIC_SHORT, HAPTIC_MEDIUM, HAPTIC_LONG } from './haptic'

describe('haptic', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
  })

  it('calls navigator.vibrate with explicit pattern', () => {
    const vibrate = vi.fn()
    Object.defineProperty(navigator, 'vibrate', { value: vibrate, configurable: true })

    haptic(50)
    expect(vibrate).toHaveBeenCalledWith(50)
  })

  it('calls navigator.vibrate with array pattern', () => {
    const vibrate = vi.fn()
    Object.defineProperty(navigator, 'vibrate', { value: vibrate, configurable: true })

    haptic(HAPTIC_LONG)
    expect(vibrate).toHaveBeenCalledWith(HAPTIC_LONG)
  })

  it('uses HAPTIC_SHORT as default when pattern is omitted', () => {
    const vibrate = vi.fn()
    Object.defineProperty(navigator, 'vibrate', { value: vibrate, configurable: true })

    haptic()
    expect(vibrate).toHaveBeenCalledWith(HAPTIC_SHORT)
  })

  it('does not throw when navigator.vibrate is absent', () => {
    const originalVibrate = (navigator as Partial<Navigator>).vibrate
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (navigator as any).vibrate

    expect(() => haptic()).not.toThrow()

    if (originalVibrate !== undefined) {
      Object.defineProperty(navigator, 'vibrate', { value: originalVibrate, configurable: true })
    }
  })

  it('exports semantic constants', () => {
    expect(HAPTIC_SHORT).toBe(10)
    expect(HAPTIC_MEDIUM).toBe(20)
    expect(HAPTIC_LONG).toEqual([10, 30, 10])
  })
})
