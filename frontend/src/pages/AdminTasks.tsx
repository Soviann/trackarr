import { useState } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import s from './AdminTasks.module.css'

interface Task {
  id: number
  task_type: string
  payload: string
  status: string
  attempts: number
  max_attempts: number
  day: number
  last_error: string | null
  run_at: string
  created_at: string
}

interface TasksResponse {
  pending: Task[] | null
  dead: Task[] | null
}

const typeLabels: Record<string, string> = {
  enrichment: 'Enrichissement',
  refresh: 'Rafraîchissement',
  cover_fetch: 'Couverture',
}

function parseTitleFromPayload(payload: string): string {
  try {
    const p = JSON.parse(payload)
    return p.title_name || `Titre #${p.title_id}`
  } catch {
    return 'Inconnu'
  }
}

function formatRelativeTime(dateStr: string): string {
  const d = new Date(dateStr)
  const now = new Date()
  const diffMs = d.getTime() - now.getTime()
  const absDiffMs = Math.abs(diffMs)

  if (absDiffMs < 60_000) return 'maintenant'
  if (absDiffMs < 3_600_000) {
    const mins = Math.round(absDiffMs / 60_000)
    return diffMs > 0 ? `dans ${mins}min` : `il y a ${mins}min`
  }
  if (absDiffMs < 86_400_000) {
    const hours = Math.round(absDiffMs / 3_600_000)
    return diffMs > 0 ? `dans ${hours}h` : `il y a ${hours}h`
  }
  return d.toLocaleDateString('fr-FR', { day: 'numeric', month: 'short' })
}

export function AdminTasks({ path }: { path?: string }) {
  const { data, loading, mutate } = useApi<TasksResponse>('/admin/tasks')
  const [acting, setActing] = useState<number | null>(null)

  const handleRetry = async (id: number) => {
    setActing(id)
    try {
      await apiFetch(`/admin/tasks/${id}/retry`, { method: 'POST' })
      mutate()
    } finally {
      setActing(null)
    }
  }

  const handleDelete = async (id: number) => {
    setActing(id)
    try {
      await apiFetch(`/admin/tasks/${id}`, { method: 'DELETE' })
      mutate()
    } finally {
      setActing(null)
    }
  }

  const pending = data?.pending ?? []
  const dead = data?.dead ?? []

  return (
    <div className={s.page}>
      <div className={s.header}>
        <div onClick={() => history.back()} className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
        </div>
        <h1 className={s.title}>Tâches</h1>
      </div>

      {loading && <div className={s.loading}>Chargement...</div>}

      {!loading && pending.length === 0 && dead.length === 0 && (
        <div className={s.empty}>Aucune tâche en attente ou en erreur</div>
      )}

      {pending.length > 0 && (
        <section className={s.section}>
          <h2 className={s.sectionLabel}>En attente</h2>
          {pending.map((task) => (
            <div key={task.id} className={s.taskCard}>
              <div className={s.taskType}>{typeLabels[task.task_type] ?? task.task_type}</div>
              <div className={s.taskTitle}>{parseTitleFromPayload(task.payload)}</div>
              <div className={s.taskMeta}>
                {task.attempts}/{task.max_attempts} — jour {task.day} · {formatRelativeTime(task.run_at)}
              </div>
              {task.last_error && (
                <div className={s.taskError}>{task.last_error}</div>
              )}
            </div>
          ))}
        </section>
      )}

      {dead.length > 0 && (
        <section className={s.section}>
          <h2 className={s.sectionLabelDead}>Échouées</h2>
          {dead.map((task) => (
            <div key={task.id} className={s.taskCard}>
              <div className={s.taskType}>{typeLabels[task.task_type] ?? task.task_type}</div>
              <div className={s.taskTitle}>{parseTitleFromPayload(task.payload)}</div>
              {task.last_error && (
                <div className={s.taskError}>{task.last_error}</div>
              )}
              <div className={s.taskActions}>
                <button
                  className={s.retryBtn}
                  onClick={() => handleRetry(task.id)}
                  disabled={acting === task.id}
                >
                  Relancer
                </button>
                <button
                  className={s.deleteBtn}
                  onClick={() => handleDelete(task.id)}
                  disabled={acting === task.id}
                >
                  Supprimer
                </button>
              </div>
            </div>
          ))}
        </section>
      )}
    </div>
  )
}
