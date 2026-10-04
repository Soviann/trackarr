import clsx from 'clsx'
import { useTranslation, type TranslationKey } from '../i18n'
import type { TitleStatus } from '../types'
import s from './StatusBadge.module.css'

const statusKeyMap: Record<TitleStatus, TranslationKey> = {
  watching: 'status.watching',
  completed: 'status.completed',
  dropped: 'status.dropped',
  plan_to_watch: 'status.planToWatch',
}

const statusClass: Record<TitleStatus, string> = {
  watching: s.watching,
  completed: s.completed,
  dropped: s.dropped,
  plan_to_watch: s.planToWatch,
}

export function StatusBadge({ status, caughtUp }: { status: TitleStatus; caughtUp?: boolean }) {
  const { t } = useTranslation()
  const isCaughtUp = status === 'watching' && caughtUp
  const label = isCaughtUp ? t('status.caughtUp') : t(statusKeyMap[status])

  return (
    <span class={clsx(s.badge, isCaughtUp ? s.caughtUp : statusClass[status])}>
      {label}
    </span>
  )
}
