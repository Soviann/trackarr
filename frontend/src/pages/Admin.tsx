import { route } from 'preact-router'
import { useApi } from '../hooks/useApi'
import { colors } from '../theme'
import s from './Admin.module.css'

interface AdminCounts {
  pending_validations: number
  dead_tasks: number
}

const cards = [
  {
    id: 'validations',
    label: 'Validations',
    description: 'Titres en attente de validation',
    path: '/admin/validate',
    color: colors.accentAmber,
    countKey: 'pending_validations' as const,
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14" />
        <polyline points="22 4 12 14.01 9 11.01" />
      </svg>
    ),
  },
  {
    id: 'tasks',
    label: 'Tâches',
    description: 'File d\'attente et erreurs',
    path: '/admin/tasks',
    color: colors.accentCoral,
    countKey: 'dead_tasks' as const,
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <polyline points="22 12 18 12 15 21 9 3 6 12 2 12" />
      </svg>
    ),
  },
  {
    id: 'notifications',
    label: 'Notifications',
    description: 'Gérer les préférences',
    path: '/admin/notifications',
    color: colors.accentBlue,
    countKey: null,
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
        <path d="M13.73 21a2 2 0 0 1-3.46 0" />
      </svg>
    ),
  },
]

export function Admin({ path }: { path?: string }) {
  const { data: counts } = useApi<AdminCounts>('/admin/counts')

  return (
    <div className={s.page}>
      <h1 className={s.title}>Administration</h1>
      <div className={s.grid}>
        {cards.map((card) => {
          const count = card.countKey && counts ? counts[card.countKey] : null
          return (
            <button
              key={card.id}
              className={s.card}
              onClick={() => route(card.path)}
              style={{ '--card-color': card.color } as Record<string, string>}
            >
              <div className={s.cardIcon}>{card.icon}</div>
              <div className={s.cardContent}>
                <div className={s.cardLabel}>
                  {card.label}
                  {count != null && count > 0 && (
                    <span className={s.badge}>{count}</span>
                  )}
                </div>
                <div className={s.cardDesc}>{card.description}</div>
              </div>
              <svg className={s.cardArrow} width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <polyline points="9 18 15 12 9 6" />
              </svg>
            </button>
          )
        })}
      </div>
    </div>
  )
}
