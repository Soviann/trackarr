import type { ComponentChildren } from 'preact'
import clsx from 'clsx'
import { useTranslation } from '../i18n'
import { navTabs, getActiveTab } from './navItems'
import s from './Navbar.module.css'

interface NavbarProps {
  currentPath: string
  onNavigate: (path: string) => void
  above?: ComponentChildren
}

export function Navbar({ currentPath, onNavigate, above }: NavbarProps) {
  const { t } = useTranslation()
  const activePath = getActiveTab(currentPath)

  return (
    <div className={clsx(s.wrapper, !above && s.noAbove)}>
      {above && <div className={s.above}>{above}</div>}
      <nav className={s.nav} aria-label={t('nav.mainNav')}>
        {navTabs.map((tab) => {
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
