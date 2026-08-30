import { describe, it, expect, beforeEach } from 'vitest'
import {
  ALL_WATCH_PROVIDERS,
  DEFAULT_ENABLED_PROVIDERS,
  getEnabledWatchProviders,
  setEnabledWatchProviders,
  getMatchingProviders,
  isOnPrime,
} from './providers'

describe('providers utils', () => {
  beforeEach(() => {
    localStorage.clear()
    setEnabledWatchProviders('netflix,prime,disney,apple,max,canal,crunchyroll,paramount,adn')
  })

  it('has 9 recognized watch providers', () => {
    expect(ALL_WATCH_PROVIDERS.length).toBe(9)
    expect(DEFAULT_ENABLED_PROVIDERS).toBe('netflix,prime,disney,apple,max,canal,crunchyroll,paramount,adn')
  })

  it('enables and disables providers correctly', () => {
    setEnabledWatchProviders('netflix,prime')
    const active = getEnabledWatchProviders()
    expect(active.has('netflix')).toBe(true)
    expect(active.has('prime')).toBe(true)
    expect(active.has('disney')).toBe(false)
  })

  it('correctly matches TMDB providers against enabled set', () => {
    const tmdbProviders = [
      { id: 8, name: 'Netflix' },
      { id: 381, name: 'Canal+' },
      { id: 9999, name: 'Unknown Streamer' },
    ]
    const matched = getMatchingProviders(tmdbProviders)
    expect(matched.map((m) => m.id)).toEqual(['netflix', 'canal'])
  })

  it('isOnPrime handles 9 and 119 while rejecting other IDs', () => {
    expect(isOnPrime([{ id: 9, name: 'Amazon Prime Video' }])).toBe(true)
    expect(isOnPrime([{ id: 119, name: 'Amazon Prime Video' }])).toBe(true)
    expect(isOnPrime([{ id: 10, name: 'Amazon Video' }])).toBe(false)
    expect(isOnPrime([])).toBe(false)
    expect(isOnPrime(undefined)).toBe(false)
  })
})
