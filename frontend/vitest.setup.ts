const fallbackCancelAnimationFrame = (id: number) => {
  if (typeof window !== 'undefined' && typeof window.cancelAnimationFrame === 'function' && window.cancelAnimationFrame !== fallbackCancelAnimationFrame) {
    try {
      window.cancelAnimationFrame(id)
      return
    } catch {
      // JSDOM window might be destroyed during teardown
    }
  }
  clearTimeout(id)
}

if (typeof globalThis !== 'undefined') {
  globalThis.cancelAnimationFrame = fallbackCancelAnimationFrame
}
if (typeof global !== 'undefined') {
  (global as any).cancelAnimationFrame = fallbackCancelAnimationFrame
}
if (typeof window !== 'undefined') {
  (window as any).cancelAnimationFrame = fallbackCancelAnimationFrame
}
