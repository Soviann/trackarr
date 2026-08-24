import { useEffect, useRef } from 'preact/hooks'

const scrollPositions = new Map<string, number>()

export function clearSavedScroll(key: string) {
  scrollPositions.delete(key)
  try {
    sessionStorage.removeItem(`scroll_${key}`)
  } catch {
    // Ignore sessionStorage errors
  }
}

export function getSavedScroll(key: string): number | undefined {
  if (scrollPositions.has(key)) return scrollPositions.get(key)
  try {
    const val = sessionStorage.getItem(`scroll_${key}`)
    if (val !== null) {
      const num = parseInt(val, 10)
      if (!isNaN(num)) return num
    }
  } catch {
    // Ignore sessionStorage errors
  }
  return undefined
}

export function saveScroll(key: string, y: number) {
  scrollPositions.set(key, y)
  try {
    sessionStorage.setItem(`scroll_${key}`, String(y))
  } catch {
    // Ignore sessionStorage errors
  }
}

export function useScrollRestoration(key: string, isReady: boolean = true) {
  const isRestoring = useRef(false)
  const currentYRef = useRef<number | undefined>(undefined)

  useEffect(() => {
    const savedY = getSavedScroll(key)
    const originalPath = window.location.pathname

    if (savedY !== undefined && savedY > 0) {
      currentYRef.current = savedY
    } else {
      currentYRef.current = 0
      isRestoring.current = true
      window.scrollTo(0, 0)
      setTimeout(() => {
        isRestoring.current = false
      }, 50)
    }

    const onScroll = () => {
      if (!isRestoring.current && isReady && window.location.pathname === originalPath) {
        currentYRef.current = window.scrollY
        saveScroll(key, window.scrollY)
      }
    }

    window.addEventListener('scroll', onScroll, { passive: true })

    return () => {
      window.removeEventListener('scroll', onScroll)
      if (isReady && currentYRef.current !== undefined && window.location.pathname === originalPath) {
        saveScroll(key, currentYRef.current)
      }
    }
  }, [key, isReady])

  useEffect(() => {
    if (!isReady) return

    const savedY = getSavedScroll(key)
    const targetY = (savedY !== undefined && savedY > 0) ? savedY : 0

    isRestoring.current = true
    let attempts = 0
    let rafId: number

    const tryScroll = () => {
      window.scrollTo(0, targetY)
      const maxScroll = Math.max(0, document.documentElement.scrollHeight - window.innerHeight)
      const reached = Math.abs(window.scrollY - targetY) <= 5 || window.scrollY >= maxScroll

      if (!reached && attempts < 10) {
        attempts++
        rafId = requestAnimationFrame(tryScroll)
      } else {
        setTimeout(() => {
          isRestoring.current = false
        }, 50)
      }
    }

    rafId = requestAnimationFrame(tryScroll)

    return () => {
      cancelAnimationFrame(rafId)
      isRestoring.current = false
    }
  }, [key, isReady])
}
