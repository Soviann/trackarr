export const HAPTIC_SHORT = 10
export const HAPTIC_MEDIUM = 20
export const HAPTIC_LONG = [10, 30, 10]

export function haptic(pattern?: number | number[]): void {
  if ('vibrate' in navigator) {
    navigator.vibrate(pattern ?? HAPTIC_SHORT)
  }
}
