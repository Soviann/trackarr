import clsx from 'clsx'
import type { TitleStatus } from '../types'
import s from './StatusBadge.module.css'

const statusLabels: Record<TitleStatus, string> = {
  watching: 'WATCHING',
  completed: 'COMPLETED',
  dropped: 'DROPPED',
  plan_to_watch: 'PLAN',
}

const statusClass: Record<TitleStatus, string> = {
  watching: s.watching,
  completed: s.completed,
  dropped: s.dropped,
  plan_to_watch: s.planToWatch,
}

export function StatusBadge({ status, caughtUp }: { status: TitleStatus; caughtUp?: boolean }) {
  const isCaughtUp = status === 'watching' && caughtUp
  return (
    <span class={clsx(s.badge, isCaughtUp ? s.caughtUp : statusClass[status])}>
      {isCaughtUp ? 'CAUGHT UP' : statusLabels[status]}
    </span>
  )
}
