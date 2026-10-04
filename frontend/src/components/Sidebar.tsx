import clsx from 'clsx'
import { useTranslation } from '../i18n'
import { navTabs, getActiveTab } from './navItems'
import s from './Sidebar.module.css'

interface SidebarProps {
  currentPath: string
  onNavigate: (path: string) => void
  collapsed: boolean
  onToggleCollapse: () => void
}

export function Sidebar({ currentPath, onNavigate, collapsed, onToggleCollapse }: SidebarProps) {
  const { t } = useTranslation()
  const activePath = getActiveTab(currentPath)

  return (
    <aside
      className={clsx(s.sidebar, collapsed && s.collapsed)}
      aria-label="Desktop Navigation"
    >
      <div className={s.header}>
        <button
          type="button"
          className={s.brandArea}
          onClick={() => onNavigate('/')}
          title="Trackarr"
          aria-label="Trackarr"
        >
          {collapsed ? (
            <span className={s.brandLogoMini}>
              T<span className={s.brandAccent}>a</span>
            </span>
          ) : (
            <div className={s.brandLogoFull}>
              <span className={s.brandLogo}>
                Track<span className={s.brandAccent}>arr</span>
              </span>
              <span className={s.brandSub}>MEDIA</span>
            </div>
          )}
        </button>
      </div>

      <nav className={s.nav}>
        {navTabs.map((tab) => {
          const active = activePath === tab.path
          const label = t(tab.labelKey)

          return (
            <button
              type="button"
              key={tab.id}
              onClick={() => onNavigate(tab.path)}
              className={clsx(s.tab, active && s.tabActive)}
              aria-label={label}
              aria-current={active ? 'page' : undefined}
              title={collapsed ? label : undefined}
            >
              <span className={s.iconWrapper}>{tab.icon}</span>
              {!collapsed && <span className={s.label}>{label}</span>}
              {active && <span className={s.activeBar} aria-hidden="true" />}
            </button>
          )
        })}
      </nav>

      <div className={s.footer}>
        <button
          type="button"
          onClick={onToggleCollapse}
          className={s.collapseBtn}
          aria-label={collapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')}
          title={collapsed ? t('nav.expandSidebar') : t('nav.collapseSidebar')}
        >
          <svg
            width="18"
            height="18"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            stroke-width="2"
            stroke-linecap="round"
            stroke-linejoin="round"
            className={clsx(s.collapseIcon, collapsed && s.collapseIconCollapsed)}
            aria-hidden="true"
          >
            <polyline points="15 18 9 12 15 6" />
          </svg>
          {!collapsed && <span className={s.collapseLabel}>{t('nav.collapseSidebar')}</span>}
        </button>
      </div>
    </aside>
  )
}
