import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { colors } from '../theme'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import s from './Admin.module.css'

interface AdminCounts {
  pending_validations: number
  dead_tasks: number
}

const cards = [
  {
    id: 'validations',
    label: 'Validations',
    description: 'Titles pending validation',
    path: '/admin/validate',
    color: colors.accent,
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
    label: 'Tasks',
    description: 'Queue and errors',
    path: '/admin/tasks',
    color: colors.statusCrit,
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
    description: 'Manage preferences',
    path: '/admin/notifications',
    color: colors.accent,
    countKey: null,
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
        <path d="M13.73 21a2 2 0 0 1-3.46 0" />
      </svg>
    ),
  },
  {
    id: 'anilist',
    label: 'AniList',
    description: 'Connection & sync status',
    path: '/admin/anilist',
    color: colors.brandAnilist,
    countKey: null,
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71" />
        <path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71" />
      </svg>
    ),
  },
  {
    id: 'help',
    label: 'Help',
    description: 'How does this app work?',
    path: '/admin/help',
    color: colors.accent,
    countKey: null,
    icon: (
      <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <circle cx="12" cy="12" r="10" />
        <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3" />
        <line x1="12" y1="17" x2="12.01" y2="17" />
      </svg>
    ),
  },
]

export function Admin({ path }: { path?: string }) {
  const { data: counts } = useApi<AdminCounts>('/admin/counts')
  const [refreshing, setRefreshing] = useState(false)
  const [showRefreshModal, setShowRefreshModal] = useState(false)

  const handleRefreshAll = async () => {
    setRefreshing(true)
    try {
      await apiFetch('/admin/refresh-all', { method: 'POST' })
    } finally {
      setRefreshing(false)
    }
  }

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

      <div className={s.actions}>
        <button className={s.actionBtn} onClick={() => setShowRefreshModal(true)} disabled={refreshing}>
          <div className={s.actionIcon}>
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <polyline points="23 4 23 10 17 10" />
              <polyline points="1 20 1 14 7 14" />
              <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
            </svg>
          </div>
          <div>
            <div className={s.actionLabel}>{refreshing ? 'Refresh started...' : 'Refresh all metadata'}</div>
            <div className={s.actionDesc}>Updates synopsis, genres, cast and ratings from TMDB/AniList</div>
          </div>
        </button>
      </div>

      <ConfirmationDrawer
        open={showRefreshModal}
        onClose={() => setShowRefreshModal(false)}
        onConfirm={handleRefreshAll}
        title="Refresh metadata?"
        description="This operation runs in the background and may take several minutes."
        confirmText="Refresh"
        cancelText="Cancel"
      />
    </div>
  )
}

