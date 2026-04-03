import type { TitleStatus } from '../types'
import { colors } from '../theme'

const badgeStyles: Record<TitleStatus, { color: string; bg: string; label: string }> = {
  watching: { color: colors.bgPrimary, bg: colors.accentAmber, label: 'WATCHING' },
  completed: { color: colors.accentGreen, bg: `${colors.accentGreen}1F`, label: 'COMPLETED' },
  dropped: { color: colors.accentCoral, bg: `${colors.accentCoral}1F`, label: 'DROPPED' },
  plan_to_watch: { color: colors.textSecondary, bg: colors.bgSurface, label: 'PLAN' },
}

export function StatusBadge({ status }: { status: TitleStatus }) {
  const s = badgeStyles[status]
  return (
    <span style={{
      fontSize: '9px',
      color: s.color,
      background: s.bg,
      borderRadius: '4px',
      padding: '1px 5px',
      fontWeight: 600,
      whiteSpace: 'nowrap',
    }}>
      {s.label}
    </span>
  )
}
