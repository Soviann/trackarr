import { apiFetch } from '../api'

declare global {
  interface Navigator {
    setAppBadge(count?: number): Promise<void>
    clearAppBadge(): Promise<void>
  }
}

const BADGE_ENABLED_KEY = 'badge-enabled'

export function isBadgeEnabled(): boolean {
  return localStorage.getItem(BADGE_ENABLED_KEY) !== 'false' // default true
}

export function setBadgeEnabled(enabled: boolean): void {
  localStorage.setItem(BADGE_ENABLED_KEY, String(enabled))
  if (!enabled) clearBadge()
}

export async function updateBadge(): Promise<void> {
  if (!('setAppBadge' in navigator) || !isBadgeEnabled()) return
  try {
    const data = await apiFetch<{ count: number }>('/titles/review-count')
    if (data.count > 0) {
      await navigator.setAppBadge(data.count)
    } else {
      await navigator.clearAppBadge()
    }
  } catch {
    // Silently fail — badge is a nice-to-have
  }
}

export async function clearBadge(): Promise<void> {
  if ('clearAppBadge' in navigator) {
    try { await navigator.clearAppBadge() } catch {}
  }
}
