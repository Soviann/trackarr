import type { ComponentChildren } from 'preact'
import clsx from 'clsx'
import { colors, accentWash } from '../theme'
import s from './Navbar.module.css'

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
  {
    id: 'admin',
    label: 'Admin',
    path: '/admin',
    color: colors.accentBlue,
    icon: (c: string) => (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke={c} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
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
    <div className={s.wrapper}>
      {above && <div className={s.above}>{above}</div>}
      <nav className={s.nav}>
      {tabs.map((tab) => {
        const active = activePath === tab.path
        const iconColor = active ? tab.color : colors.textMuted

        return (
          <button
            key={tab.id}
            onClick={() => onNavigate(tab.path)}
            className={clsx(s.tab, active && s.tabActive)}
            style={active ? {
              '--tab-color': tab.color,
              '--tab-wash': accentWash(tab.color),
            } as Record<string, string> : undefined}
          >
            {tab.icon(iconColor)}
            <span className={clsx(s.label, active && s.labelActive)}>
              {tab.label}
            </span>
          </button>
        )
      })}
    </nav>
    </div>
  )
}
