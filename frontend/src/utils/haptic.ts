export const HAPTIC_SHORT = 10
export const HAPTIC_MEDIUM = 20
export const HAPTIC_LONG = [10, 30, 10] as const

export function haptic(pattern?: number | readonly number[]): void {
  if ('vibrate' in navigator) {
    const value = pattern ?? HAPTIC_SHORT
    navigator.vibrate(Array.isArray(value) ? Array.from(value) : value)
  }
}
