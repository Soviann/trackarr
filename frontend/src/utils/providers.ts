import type { WatchProvider } from '../types'

// TMDB provider ids for Amazon Prime Video (subscription-included). Multiple ids
// exist across regions/history; id 10 ("Amazon Video", rent/buy) is deliberately excluded.
export const PRIME_PROVIDER_IDS: ReadonlySet<number> = new Set([9, 119])

export function isOnPrime(providers?: WatchProvider[] | null): boolean {
  return (providers ?? []).some((p) => PRIME_PROVIDER_IDS.has(p.id))
}
