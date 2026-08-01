import { useEffect, useRef } from 'preact/hooks'

const scrollPositions = new Map<string, number>()

export function useScrollRestoration(key: string) {
  const isRestoring = useRef(false)

  useEffect(() => {
    // Restore scroll position if it exists
    const savedY = scrollPositions.get(key)
    let currentY = window.scrollY
    const originalPath = window.location.pathname

    if (savedY !== undefined) {
      currentY = savedY // Initialize to saved position so we don't save 0 if user leaves without scrolling
      isRestoring.current = true
      window.scrollTo(0, savedY)
      // Wait a frame to ensure Preact has flushed DOM updates
      requestAnimationFrame(() => {
        window.scrollTo(0, savedY)
        setTimeout(() => {
          isRestoring.current = false
        }, 50)
      })
    }
    
    const onScroll = () => {
      // Don't save the 0 position if we're in the middle of restoring a saved position
      // Also ignore if the route has changed, to prevent saving 0 when DOM shrinks on navigation
      if (!isRestoring.current && window.location.pathname === originalPath) {
        currentY = window.scrollY
      }
    }

    window.addEventListener('scroll', onScroll, { passive: true })

    // On unmount, save the last known scroll position
    return () => {
      window.removeEventListener('scroll', onScroll)
      scrollPositions.set(key, currentY)
    }
  }, [key])
}
