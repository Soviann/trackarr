import { useState, useMemo } from 'preact/hooks'
import clsx from 'clsx'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
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
  enrichment: 'Enrichment',
  refresh: 'Refresh',
  cover_fetch: 'Cover',
}

function parseTitleFromPayload(payload: string): string {
  try {
    const p = JSON.parse(payload)
    return p.title_name || `Title #${p.title_id}`
  } catch {
    return 'Unknown'
  }
}

function formatRelativeTime(dateStr: string): string {
  const d = new Date(dateStr)
  const now = new Date()
  const diffMs = d.getTime() - now.getTime()
  const absDiffMs = Math.abs(diffMs)

  if (absDiffMs < 60_000) return 'now'
  if (absDiffMs < 3_600_000) {
    const mins = Math.round(absDiffMs / 60_000)
    return diffMs > 0 ? `in ${mins}m` : `${mins}m ago`
  }
  if (absDiffMs < 86_400_000) {
    const hours = Math.round(absDiffMs / 3_600_000)
    return diffMs > 0 ? `in ${hours}h` : `${hours}h ago`
  }
  return d.toLocaleDateString('en-US', { day: 'numeric', month: 'short' })
}

type FilterType = 'all' | 'pending' | 'errored'

export function AdminTasks({ path }: { path?: string }) {
  const { data, loading, mutate } = useApi<TasksResponse>('/admin/tasks')
  const [acting, setActing] = useState<number | null>(null)
  
  const [filter, setFilter] = useState<FilterType>('all')
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [isSelectMode, setIsSelectMode] = useState(false)

  // Modal state
  const [modalOpen, setModalOpen] = useState(false)
  const [modalMode, setModalMode] = useState<'batch' | number | null>(null)

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
      setSelectedIds(prev => {
        const next = new Set(prev)
        next.delete(id)
        return next
      })
      mutate()
    } finally {
      setActing(null)
    }
  }

  const handleBatchDelete = async () => {
    if (selectedIds.size === 0) return
    setActing(-1)
    try {
      await apiFetch(`/admin/tasks/batch-delete`, {
        method: 'POST',
        body: JSON.stringify({ ids: Array.from(selectedIds) })
      })
      setSelectedIds(new Set())
      mutate()
    } finally {
      setActing(null)
    }
  }

  const confirmDelete = () => {
    if (modalMode === 'batch') {
      handleBatchDelete()
    } else if (typeof modalMode === 'number') {
      handleDelete(modalMode)
    }
    setModalOpen(false)
  }

  const openDeleteModal = (mode: 'batch' | number) => {
    setModalMode(mode)
    setModalOpen(true)
  }

  const allPending = data?.pending ?? []
  const allDead = data?.dead ?? []
  const allTasks = [...allPending, ...allDead]

  const filteredTasks = useMemo(() => {
    return allTasks.filter(t => {
      if (filter === 'all') return true
      if (filter === 'pending') return t.status !== 'dead' && t.last_error === null
      if (filter === 'errored') return t.status === 'dead' || t.last_error !== null
      return true
    })
  }, [allTasks, filter])

  const toggleSelection = (id: number) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (selectedIds.size === filteredTasks.length && filteredTasks.length > 0) {
      setSelectedIds(new Set())
    } else {
      setSelectedIds(new Set(filteredTasks.map(t => t.id)))
    }
  }

  const handleSelectToggle = () => {
    if (isSelectMode) {
      setSelectedIds(new Set())
    }
    setIsSelectMode(!isSelectMode)
  }

  const allSelected = filteredTasks.length > 0 && selectedIds.size === filteredTasks.length

  return (
    <>
      <div className={s.page}>
        <div className={s.header}>
          <div className={s.headerLeft}>
            <div onClick={() => history.back()} className={s.backBtn}>
              <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
              </svg>
            </div>
            <h1 className={s.title}>Tasks</h1>
          </div>
          <button 
            className={s.selectToggleBtn} 
            data-active={isSelectMode}
            onClick={handleSelectToggle}
          >
            {isSelectMode ? 'Cancel' : 'Select'}
          </button>
        </div>

        <div className={s.filterBar}>
          <button className={s.filterBtn} data-active={filter === 'all'} onClick={() => setFilter('all')}>All</button>
          <button className={s.filterBtn} data-active={filter === 'pending'} onClick={() => setFilter('pending')}>Healthy</button>
          <button className={s.filterBtn} data-active={filter === 'errored'} onClick={() => setFilter('errored')}>Errored</button>
        </div>

        {loading && <div className={s.loading}>Loading...</div>}

        {!loading && allTasks.length === 0 && (
          <div className={s.empty}>No pending or errored tasks</div>
        )}

        {!loading && allTasks.length > 0 && filteredTasks.length === 0 && (
          <div className={s.empty}>No tasks match this filter</div>
        )}

        {filteredTasks.length > 0 && (
          <>
            {isSelectMode && (
              <div className={s.selectionActions}>
                <button className={s.selectionBtn} onClick={toggleSelectAll}>
                  {allSelected ? 'Unselect All' : 'Select All'}
                </button>
              </div>
            )}

            <section className={s.section}>
              {filteredTasks.map((task) => {
                const isErrored = task.status === 'dead' || task.last_error !== null
                const isDead = task.status === 'dead'
                const isSelected = selectedIds.has(task.id)

                return (
                  <div 
                    key={task.id} 
                    className={clsx(s.taskCard, isSelected && s.taskCardSelected)}
                    onClick={() => isSelectMode && toggleSelection(task.id)}
                  >
                    {isSelectMode && (
                      <div className={s.checkboxContainer}>
                        <div className={clsx(s.customCheckbox, isSelected && s.customCheckboxChecked)}>
                          {isSelected && (
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="4" stroke-linecap="round" stroke-linejoin="round">
                              <polyline points="20 6 9 17 4 12" />
                            </svg>
                          )}
                        </div>
                      </div>
                    )}
                    <div className={s.taskContent}>
                      <div className={s.taskHeader}>
                        <span className={s.taskType}>{typeLabels[task.task_type] ?? task.task_type}</span>
                        <span className={isDead ? s.badgeDead : s.badgePending}>
                          {isDead ? 'Failed' : 'Pending'}
                        </span>
                      </div>
                      <div className={s.taskTitle}>{parseTitleFromPayload(task.payload)}</div>
                      <div className={s.taskMeta}>
                        {task.attempts}/{task.max_attempts} — day {task.day} · {formatRelativeTime(task.run_at)}
                      </div>
                      {task.last_error && (
                        <div className={s.taskError}>{task.last_error}</div>
                      )}
                      {isErrored && (
                        <div className={s.taskActions}>
                          {isDead && (
                            <button
                              className={s.retryBtn}
                              onClick={(e) => { e.stopPropagation(); handleRetry(task.id); }}
                              disabled={acting === task.id || acting === -1}
                            >
                              Retry
                            </button>
                          )}
                          <button
                            className={s.deleteBtn}
                            onClick={(e) => { e.stopPropagation(); openDeleteModal(task.id); }}
                            disabled={acting === task.id || acting === -1}
                          >
                            Delete
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </section>
          </>
        )}
      </div>

      {selectedIds.size > 0 && (
        <div className={s.actionBar}>
          <span className={s.actionText}>{selectedIds.size} selected</span>
          <button 
            className={s.batchDeleteBtn}
            onClick={() => openDeleteModal('batch')}
            disabled={acting === -1}
          >
            Delete
          </button>
        </div>
      )}

      <ConfirmationDrawer
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onConfirm={confirmDelete}
        title={modalMode === 'batch' 
          ? `Supprimer ${selectedIds.size} tâches ?`
          : 'Supprimer cette tâche ?'}
        confirmText="Supprimer"
        cancelText="Annuler"
        isDangerous
      />
      </>
      )
      }