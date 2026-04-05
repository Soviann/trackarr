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

export function StatusBadge({ status }: { status: TitleStatus }) {
  return (
    <span class={clsx(s.badge, statusClass[status])}>
      {statusLabels[status]}
    </span>
  )
}
