import type { ComponentChildren, JSX } from 'preact'
import { route } from 'preact-router'
import { clsx } from 'clsx'
import { routeTo } from '../routes'
import { useTranslation } from '../i18n'
import s from './AdminHeader.module.css'

export interface AdminHeaderProps {
  title: ComponentChildren
  backTo?: string
  onBack?: () => void
  badge?: ComponentChildren
  children?: ComponentChildren
  className?: string
}

export function AdminHeader({
  title,
  backTo,
  onBack,
  badge,
  children,
  className,
}: AdminHeaderProps): JSX.Element {
  const { t } = useTranslation()

  const handleBack = () => {
    if (onBack) {
      onBack()
    } else if (backTo) {
      route(backTo)
    } else {
      route(routeTo.admin())
    }
  }

  const backLabel = t('common.back')

  return (
    <header className={clsx(s.header, className)}>
      <div className={s.left}>
        <button
          type="button"
          onClick={handleBack}
          className={s.backBtn}
          aria-label={backLabel}
          title={backLabel}
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
            aria-hidden="true"
          >
            <line x1="19" y1="12" x2="5" y2="12" />
            <polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
        <h1 className={s.title}>{title}</h1>
        {badge && <div className={s.badge}>{badge}</div>}
      </div>
      {children && <div className={s.actions}>{children}</div>}
    </header>
  )
}
