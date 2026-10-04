import type { ComponentChildren } from 'preact'
import clsx from 'clsx'
import { useTranslation, type TranslationKey } from '../i18n'
import { ROUTE_PATHS } from '../routes'
import s from './Navbar.module.css'

interface NavTab {
  id: string
  labelKey: TranslationKey
  path: string
  icon: ComponentChildren
}

const tabs: NavTab[] = [
  {
    id: 'collection',
    labelKey: 'nav.collection',
    path: ROUTE_PATHS.home,
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
        <line x1="8" y1="21" x2="16" y2="21" />
        <line x1="12" y1="17" x2="12" y2="21" />
      </svg>
    ),
  },
  {
    id: 'explore',
    labelKey: 'nav.explore',
    path: ROUTE_PATHS.search,
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="11" cy="11" r="8" />
        <line x1="21" y1="21" x2="16.65" y2="16.65" />
      </svg>
    ),
  },
  {
    id: 'calendar',
    labelKey: 'nav.calendar',
    path: ROUTE_PATHS.comingUp,
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <rect x="3" y="4" width="18" height="18" rx="2" ry="2" />
        <line x1="16" y1="2" x2="16" y2="6" />
        <line x1="8" y1="2" x2="8" y2="6" />
        <line x1="3" y1="10" x2="21" y2="10" />
      </svg>
    ),
  },
  {
    id: 'stats',
    labelKey: 'nav.stats',
    path: ROUTE_PATHS.stats,
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <line x1="18" y1="20" x2="18" y2="10" />
        <line x1="12" y1="20" x2="12" y2="4" />
        <line x1="6" y1="20" x2="6" y2="14" />
      </svg>
    ),
  },
  {
    id: 'admin',
    labelKey: 'nav.admin',
    path: ROUTE_PATHS.admin,
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
      </svg>
    ),
  },
]

function getActiveTab(currentPath: string): string {
  const cleanPath = currentPath.split('?')[0] || ROUTE_PATHS.home
  const root = cleanPath === ROUTE_PATHS.home ? ROUTE_PATHS.home : `/${cleanPath.split('/')[1]}`
  if (root === ROUTE_PATHS.continueWatching) return ROUTE_PATHS.home
  if (root === ROUTE_PATHS.releases) return ROUTE_PATHS.comingUp
  return root
}

interface NavbarProps {
  currentPath: string
  onNavigate: (path: string) => void
  above?: ComponentChildren
}

export function Navbar({ currentPath, onNavigate, above }: NavbarProps) {
  const { t } = useTranslation()
  const activePath = getActiveTab(currentPath)

  return (
    <div className={s.wrapper}>
      {above && <div className={s.above}>{above}</div>}
      <nav className={s.nav} aria-label="Main Navigation">
        {tabs.map((tab) => {
          const active = activePath === tab.path
          const label = t(tab.labelKey)

          return (
            <button
              key={tab.id}
              onClick={() => onNavigate(tab.path)}
              className={clsx(s.tab, active && s.tabActive)}
              aria-label={label}
              aria-current={active ? 'page' : undefined}
            >
              {tab.icon}
              <span className={s.label}>{label}</span>
            </button>
          )
        })}
      </nav>
    </div>
  )
}
