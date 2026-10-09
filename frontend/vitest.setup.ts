import { beforeEach } from 'vitest'

const fallbackCancelAnimationFrame = (id: number) => {
  clearTimeout(id)
}

const fallbackRequestAnimationFrame = (cb: FrameRequestCallback) => {
  return setTimeout(cb, 0) as unknown as number
}

function ensurePolyfills() {
  if (typeof globalThis !== 'undefined') {
    if (typeof globalThis.cancelAnimationFrame !== 'function') {
      globalThis.cancelAnimationFrame = fallbackCancelAnimationFrame
    }
    if (typeof globalThis.requestAnimationFrame !== 'function') {
      globalThis.requestAnimationFrame = fallbackRequestAnimationFrame
    }
  }
  if (typeof global !== 'undefined') {
    if (typeof (global as any).cancelAnimationFrame !== 'function') {
      (global as any).cancelAnimationFrame = fallbackCancelAnimationFrame
    }
    if (typeof (global as any).requestAnimationFrame !== 'function') {
      (global as any).requestAnimationFrame = fallbackRequestAnimationFrame
    }
  }
  if (typeof window !== 'undefined') {
    if (typeof window.cancelAnimationFrame !== 'function') {
      window.cancelAnimationFrame = fallbackCancelAnimationFrame
    }
    if (typeof window.requestAnimationFrame !== 'function') {
      window.requestAnimationFrame = fallbackRequestAnimationFrame
    }
  }
}

ensurePolyfills()
beforeEach(() => {
  ensurePolyfills()
})
