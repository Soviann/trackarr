import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { haptic, HAPTIC_SHORT, HAPTIC_MEDIUM, HAPTIC_LONG } from './haptic'

describe('haptic', () => {
  let originalVibrate: Navigator['vibrate'] | undefined

  beforeEach(() => {
    originalVibrate = (navigator as Partial<Navigator>).vibrate
  })

  afterEach(() => {
    if (originalVibrate !== undefined) {
      Object.defineProperty(navigator, 'vibrate', { value: originalVibrate, configurable: true })
    } else {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      delete (navigator as any).vibrate
    }
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
    expect(vibrate).toHaveBeenCalledWith([10, 30, 10])
  })

  it('uses HAPTIC_SHORT as default when pattern is omitted', () => {
    const vibrate = vi.fn()
    Object.defineProperty(navigator, 'vibrate', { value: vibrate, configurable: true })

    haptic()
    expect(vibrate).toHaveBeenCalledWith(HAPTIC_SHORT)
  })

  it('does not throw when navigator.vibrate is absent', () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (navigator as any).vibrate

    expect(() => haptic()).not.toThrow()
  })

  it('exports semantic constants', () => {
    expect(HAPTIC_SHORT).toBe(10)
    expect(HAPTIC_MEDIUM).toBe(20)
    expect(HAPTIC_LONG).toEqual([10, 30, 10])
  })
})
