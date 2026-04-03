import type { ComponentChildren } from 'preact'
import { colors, accentWash } from '../theme'

const tabs = [
  {
    id: 'library',
    label: 'Library',
    path: '/',
    color: colors.accentAmber,
    icon: (c: string) => (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke={c} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
        <line x1="8" y1="21" x2="16" y2="21" />
        <line x1="12" y1="17" x2="12" y2="21" />
      </svg>
    ),
  },
  {
    id: 'search',
    label: 'Search',
    path: '/search',
    color: colors.accentTeal,
    icon: (c: string) => (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke={c} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="11" cy="11" r="8" />
        <line x1="21" y1="21" x2="16.65" y2="16.65" />
      </svg>
    ),
  },
  {
    id: 'add',
    label: 'Add',
    path: '/add',
    color: colors.accentGreen,
    icon: (c: string) => (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke={c} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10" />
        <line x1="12" y1="8" x2="12" y2="16" />
        <line x1="8" y1="12" x2="16" y2="12" />
      </svg>
    ),
  },
  {
    id: 'stats',
    label: 'Stats',
    path: '/stats',
    color: colors.accentLavender,
    icon: (c: string) => (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke={c} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <line x1="18" y1="20" x2="18" y2="10" />
        <line x1="12" y1="20" x2="12" y2="4" />
        <line x1="6" y1="20" x2="6" y2="14" />
      </svg>
    ),
  },
] as const

interface NavbarProps {
  currentPath: string
  onNavigate: (path: string) => void
  above?: ComponentChildren
}

export function Navbar({ currentPath, onNavigate, above }: NavbarProps) {
  const activePath = currentPath === '/' ? '/' : `/${currentPath.split('/')[1]}`

  return (
    <div style={{
      position: 'fixed',
      bottom: 0,
      left: 0,
      right: 0,
      zIndex: 100,
      background: colors.bgPrimary,
    }}>
      {above}
      <nav style={{
        display: 'flex',
        borderTop: `1px solid ${colors.borderSubtle}`,
        paddingBottom: 'env(safe-area-inset-bottom)',
      }}>
      {tabs.map((tab) => {
        const active = activePath === tab.path
        const iconColor = active ? tab.color : colors.textMuted

        return (
          <button
            key={tab.id}
            onClick={() => onNavigate(tab.path)}
            style={{
              flex: 1,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '4px',
              padding: '12px 0 16px',
              border: 'none',
              borderTop: `2px solid ${active ? tab.color : 'transparent'}`,
              background: active ? accentWash(tab.color) : 'transparent',
              cursor: 'pointer',
              WebkitTapHighlightColor: 'transparent',
            }}
          >
            {tab.icon(iconColor)}
            <span style={{
              fontSize: '10px',
              color: active ? tab.color : colors.textMuted,
              fontWeight: active ? 600 : 400,
            }}>
              {tab.label}
            </span>
          </button>
        )
      })}
    </nav>
    </div>
  )
}
