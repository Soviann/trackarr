import { useEffect, useRef } from 'preact/hooks'

const scrollPositions = new Map<string, number>()

export function useScrollRestoration(key: string) {
  const isRestoring = useRef(false)

  useEffect(() => {
    // Restore scroll position if it exists
    const savedY = scrollPositions.get(key)
    if (savedY !== undefined) {
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

    let currentY = window.scrollY
    
    const onScroll = () => {
      // Don't save the 0 position if we're in the middle of restoring a saved position
      if (!isRestoring.current) {
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
