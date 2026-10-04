import { useEffect } from 'preact/hooks'

export interface KeyboardShortcutsOptions {
  onSearch: () => void
  onEscape?: () => void
  enabled?: boolean
}

export function useKeyboardShortcuts({
  onSearch,
  onEscape,
  enabled = true,
}: KeyboardShortcutsOptions) {
  useEffect(() => {
    if (!enabled) return

    const handleKeyDown = (e: KeyboardEvent) => {
      // 1. Universal Search: Cmd+K (Mac) or Ctrl+K (Windows/Linux)
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        onSearch()
        return
      }

      // 2. Quick Search: '/' when not typing in editable element
      if (e.key === '/' && !e.metaKey && !e.ctrlKey && !e.altKey) {
        const target = e.target as HTMLElement | null
        const isEditable =
          target &&
          (target.tagName === 'INPUT' ||
            target.tagName === 'TEXTAREA' ||
            target.tagName === 'SELECT' ||
            target.isContentEditable)

        if (!isEditable) {
          e.preventDefault()
          onSearch()
          return
        }
      }

      // 3. Escape key
      if (e.key === 'Escape') {
        onEscape?.()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onSearch, onEscape, enabled])
}
