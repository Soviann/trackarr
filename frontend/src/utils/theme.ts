export type ThemeId = 'cyber' | 'sunset' | 'emerald' | 'vault'

export interface ThemeOption {
  id: ThemeId
  name: string
  description: string
  color: string
  gradient: string
}

export const THEMES: ThemeOption[] = [
  {
    id: 'cyber',
    name: 'Cyber Cyan',
    description: 'Electric Cyan & Violet',
    color: '#06b6d4',
    gradient: 'linear-gradient(135deg, #06b6d4, #8b5cf6)',
  },
  {
    id: 'sunset',
    name: 'Sunset Coral',
    description: 'Warm Amber, Coral & Violet',
    color: '#f43f5e',
    gradient: 'linear-gradient(135deg, #f59e0b, #f43f5e, #8b5cf6)',
  },
  {
    id: 'emerald',
    name: 'Emerald Teal',
    description: 'Mint Emerald & Azure',
    color: '#10b981',
    gradient: 'linear-gradient(135deg, #10b981, #06b6d4, #3b82f6)',
  },
  {
    id: 'vault',
    name: 'Vault Amber',
    description: 'Heritage Warm Tan & Bronze',
    color: '#d4ad7a',
    gradient: 'linear-gradient(135deg, #d4ad7a, #8f6738)',
  },
]

const THEME_STORAGE_KEY = 'trackarr_theme'

export function getStoredTheme(): ThemeId {
  if (typeof window === 'undefined') return 'cyber'
  const stored = localStorage.getItem(THEME_STORAGE_KEY) as ThemeId
  if (stored && ['cyber', 'sunset', 'emerald', 'vault'].includes(stored)) {
    return stored
  }
  return 'cyber'
}

export function applyTheme(theme: ThemeId): void {
  if (typeof document === 'undefined') return
  document.documentElement.setAttribute('data-theme', theme)
  localStorage.setItem(THEME_STORAGE_KEY, theme)

  const metaTheme = document.querySelector('meta[name="theme-color"]')
  if (metaTheme) {
    const bgColors: Record<ThemeId, string> = {
      cyber: '#090d16',
      sunset: '#120d18',
      emerald: '#081412',
      vault: '#1a1714',
    }
    metaTheme.setAttribute('content', bgColors[theme] || '#090d16')
  }
}

export function initTheme(): void {
  applyTheme(getStoredTheme())
}
