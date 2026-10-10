import { useState, useEffect } from 'preact/hooks'

const DESKTOP_MEDIA_QUERY = '(min-width: 1024px)'

export function useIsDesktop(): boolean {
  const [isDesktop, setIsDesktop] = useState<boolean>(() => {
    if (typeof window === 'undefined' || !window.matchMedia) {
      return false
    }
    return window.matchMedia(DESKTOP_MEDIA_QUERY).matches
  })

  useEffect(() => {
    if (typeof window === 'undefined' || !window.matchMedia) {
      return
    }
    const media = window.matchMedia(DESKTOP_MEDIA_QUERY)
    const update = (e: MediaQueryListEvent | MediaQueryList) => {
      setIsDesktop(e.matches)
    }
    setIsDesktop(media.matches)

    if (media.addEventListener) {
      media.addEventListener('change', update)
      return () => media.removeEventListener('change', update)
    } else {
      // Fallback for older browsers
      media.addListener(update)
      return () => media.removeListener(update)
    }
  }, [])

  return isDesktop
}
