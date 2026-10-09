import { beforeEach } from 'vitest'

const fallbackCancelAnimationFrame = (id: number) => {
  clearTimeout(id)
}

const fallbackRequestAnimationFrame = (cb: FrameRequestCallback) => {
  return setTimeout(cb, 0) as unknown as number
}

function ensurePolyfills() {
  if (typeof globalThis !== 'undefined') {
    globalThis.cancelAnimationFrame = globalThis.cancelAnimationFrame ?? fallbackCancelAnimationFrame
    globalThis.requestAnimationFrame = globalThis.requestAnimationFrame ?? fallbackRequestAnimationFrame
  }
  if (typeof global !== 'undefined') {
    ;(global as any).cancelAnimationFrame = (global as any).cancelAnimationFrame ?? fallbackCancelAnimationFrame
    ;(global as any).requestAnimationFrame = (global as any).requestAnimationFrame ?? fallbackRequestAnimationFrame
  }
  if (typeof window !== 'undefined') {
    window.cancelAnimationFrame = window.cancelAnimationFrame ?? fallbackCancelAnimationFrame
    window.requestAnimationFrame = window.requestAnimationFrame ?? fallbackRequestAnimationFrame
  }
}

ensurePolyfills()
beforeEach(() => {
  ensurePolyfills()
})
