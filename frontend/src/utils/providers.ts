import type { WatchProvider } from '../types'

export interface KnownProvider {
  id: string
  name: string
  shortName: string
  tmdbIds: number[]
  color: string
  bg: string
  border?: string
}

export const ALL_WATCH_PROVIDERS: KnownProvider[] = [
  { id: 'netflix', name: 'Netflix', shortName: 'netflix', tmdbIds: [8, 1796], color: '#ffffff', bg: '#e50914' },
  { id: 'prime', name: 'Amazon Prime Video', shortName: 'prime', tmdbIds: [9, 119], color: '#ffffff', bg: '#00a8e1' },
  { id: 'disney', name: 'Disney+', shortName: 'disney+', tmdbIds: [337], color: '#ffffff', bg: '#0063e5' },
  { id: 'apple', name: 'Apple TV+', shortName: 'apple tv+', tmdbIds: [350, 2], color: '#ffffff', bg: '#1c1c1e', border: '1px solid rgba(255, 255, 255, 0.2)' },
  { id: 'max', name: 'Max', shortName: 'max', tmdbIds: [1899, 384], color: '#ffffff', bg: '#002be7' },
  { id: 'canal', name: 'Canal+', shortName: 'canal+', tmdbIds: [381, 382, 1870], color: '#ffffff', bg: '#111111', border: '1px solid rgba(255, 255, 255, 0.25)' },
  { id: 'crunchyroll', name: 'Crunchyroll', shortName: 'crunchyroll', tmdbIds: [283], color: '#ffffff', bg: '#f47521' },
  { id: 'paramount', name: 'Paramount+', shortName: 'paramount+', tmdbIds: [531, 582], color: '#ffffff', bg: '#0064ff' },
  { id: 'adn', name: 'Animation Digital Network', shortName: 'adn', tmdbIds: [415], color: '#ffffff', bg: '#009fe3' },
]

export const DEFAULT_ENABLED_PROVIDERS = ALL_WATCH_PROVIDERS.map((p) => p.id).join(',')

const WATCH_PROVIDERS_STORAGE_KEY = 'trackarr_enabled_watch_providers'

let currentEnabledProviders: Set<string> = new Set(ALL_WATCH_PROVIDERS.map((p) => p.id))

if (typeof localStorage !== 'undefined') {
  const stored = localStorage.getItem(WATCH_PROVIDERS_STORAGE_KEY)
  if (stored !== null) {
    currentEnabledProviders = new Set(
      stored.split(',').map((s) => s.trim()).filter(Boolean)
    )
  }
}

export function getEnabledWatchProviders(): Set<string> {
  return currentEnabledProviders
}

export function setEnabledWatchProviders(csvOrList: string | string[]): void {
  const list = Array.isArray(csvOrList)
    ? csvOrList
    : csvOrList
      ? csvOrList.split(',').map((s) => s.trim()).filter(Boolean)
      : []
  currentEnabledProviders = new Set(list)
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem(WATCH_PROVIDERS_STORAGE_KEY, list.join(','))
  }
}

// TMDB provider ids for Amazon Prime Video (subscription-included). Multiple ids
// exist across regions/history; id 10 ("Amazon Video", rent/buy) is deliberately excluded.
export const PRIME_PROVIDER_IDS: ReadonlySet<number> = new Set([9, 119])

export function isOnPrime(providers?: WatchProvider[] | null): boolean {
  return (providers ?? []).some((p) => PRIME_PROVIDER_IDS.has(p.id))
}

/**
 * Returns the list of known, enabled providers that carry the title.
 */
export function getMatchingProviders(
  providers?: WatchProvider[] | null,
  enabledSet?: Set<string>
): KnownProvider[] {
  if (!providers || providers.length === 0) return []
  const activeSet = enabledSet ?? getEnabledWatchProviders()
  const tmdbIds = new Set(providers.map((p) => p.id))

  const matched: KnownProvider[] = []
  for (const kp of ALL_WATCH_PROVIDERS) {
    if (activeSet.has(kp.id) && kp.tmdbIds.some((id) => tmdbIds.has(id))) {
      matched.push(kp)
    }
  }
  return matched
}
