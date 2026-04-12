import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { updateBadge, isBadgeEnabled, setBadgeEnabled } from './badge'

vi.mock('../api', () => ({
  apiFetch: vi.fn(),
}))

import { apiFetch } from '../api'

const mockApiFetch = vi.mocked(apiFetch)

describe('badge', () => {
  let originalSetAppBadge: Navigator['setAppBadge'] | undefined
  let originalClearAppBadge: Navigator['clearAppBadge'] | undefined

  beforeEach(() => {
    originalSetAppBadge = (navigator as Partial<Navigator>).setAppBadge
    originalClearAppBadge = (navigator as Partial<Navigator>).clearAppBadge
    localStorage.clear()
    vi.clearAllMocks()
  })

  afterEach(() => {
    if (originalSetAppBadge !== undefined) {
      Object.defineProperty(navigator, 'setAppBadge', { value: originalSetAppBadge, configurable: true })
    } else {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      delete (navigator as any).setAppBadge
    }
    if (originalClearAppBadge !== undefined) {
      Object.defineProperty(navigator, 'clearAppBadge', { value: originalClearAppBadge, configurable: true })
    } else {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      delete (navigator as any).clearAppBadge
    }
  })

  function mockBadgeAPIs() {
    const setAppBadge = vi.fn().mockResolvedValue(undefined)
    const clearAppBadge = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'setAppBadge', { value: setAppBadge, configurable: true })
    Object.defineProperty(navigator, 'clearAppBadge', { value: clearAppBadge, configurable: true })
    return { setAppBadge, clearAppBadge }
  }

  it('calls setAppBadge(count) when count > 0', async () => {
    const { setAppBadge, clearAppBadge } = mockBadgeAPIs()
    mockApiFetch.mockResolvedValue({ count: 5 })

    await updateBadge()

    expect(setAppBadge).toHaveBeenCalledWith(5)
    expect(clearAppBadge).not.toHaveBeenCalled()
  })

  it('calls clearAppBadge() when count === 0', async () => {
    const { setAppBadge, clearAppBadge } = mockBadgeAPIs()
    mockApiFetch.mockResolvedValue({ count: 0 })

    await updateBadge()

    expect(clearAppBadge).toHaveBeenCalled()
    expect(setAppBadge).not.toHaveBeenCalled()
  })

  it('does not call badge API when badge-enabled is false', async () => {
    const { setAppBadge, clearAppBadge } = mockBadgeAPIs()
    localStorage.setItem('badge-enabled', 'false')
    mockApiFetch.mockResolvedValue({ count: 3 })

    await updateBadge()

    expect(setAppBadge).not.toHaveBeenCalled()
    expect(clearAppBadge).not.toHaveBeenCalled()
  })

  it('does not call badge API when setAppBadge is not in navigator', async () => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    delete (navigator as any).setAppBadge
    mockApiFetch.mockResolvedValue({ count: 3 })

    await updateBadge()

    expect(mockApiFetch).not.toHaveBeenCalled()
  })

  it('isBadgeEnabled returns true by default', () => {
    expect(isBadgeEnabled()).toBe(true)
  })

  it('setBadgeEnabled(false) sets localStorage and skips badge updates', async () => {
    const { setAppBadge } = mockBadgeAPIs()
    setBadgeEnabled(false)
    mockApiFetch.mockResolvedValue({ count: 2 })

    await updateBadge()

    expect(setAppBadge).not.toHaveBeenCalled()
    expect(isBadgeEnabled()).toBe(false)
  })

  it('silently fails on apiFetch error', async () => {
    mockBadgeAPIs()
    mockApiFetch.mockRejectedValue(new Error('network error'))

    await expect(updateBadge()).resolves.toBeUndefined()
  })
})
