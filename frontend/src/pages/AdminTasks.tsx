import { useState, useEffect, useRef } from 'preact/hooks'
import clsx from 'clsx'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { useTranslation } from '../i18n'
import { AdminHeader } from '../components/AdminHeader'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import { formatRelativeTime } from '../utils'
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
  tasks: Task[]
  total: number
}

function parseTitleFromPayload(payload: string): string {
  try {
    const p = JSON.parse(payload)
    return p.title_name || `Title #${p.title_id}`
  } catch {
    return 'Unknown'
  }
}

type FilterType = 'all' | 'pending' | 'errored'

export function AdminTasks({ path }: { path?: string }) {
  const { t } = useTranslation()
  const [acting, setActing] = useState<number | null>(null)
  const [filter, setFilter] = useState<FilterType>('all')
  const [page, setPage] = useState(1)
  const [allTasks, setAllTasks] = useState<Task[]>([])
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [isSelectMode, setIsSelectMode] = useState(false)

  useEffect(() => {
    setPage(1)
    setAllTasks([])
    setSelectedIds(new Set())
    setIsSelectMode(false)
  }, [filter])

  const limit = 50
  const offset = (page - 1) * 50
  const { data, loading, mutate: rawMutate } = useApi<TasksResponse>(`/admin/tasks?filter=${filter}&limit=${limit}&offset=${offset}`)
  const pageRef = useRef(page)
  pageRef.current = page

  useEffect(() => {
    if (!data?.tasks) return
    if (pageRef.current === 1) {
      setAllTasks(data.tasks)
    } else {
      setAllTasks(prev => [...prev, ...data.tasks])
    }
  }, [data])

  const mutate = () => {
    setAllTasks([])
    if (page === 1) {
      rawMutate()
    } else {
      setPage(1)
    }
  }

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

  const filteredTasks = allTasks
  const total = data?.total ?? 0
  const hasMore = allTasks.length < total

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

  const getTaskTypeLabel = (type: string): string => {
    switch (type) {
      case 'enrichment':
        return t('adminTasks.typeEnrichment')
      case 'refresh':
        return t('adminTasks.typeRefresh')
      case 'cover_fetch':
        return t('adminTasks.typeCover')
      default:
        return type
    }
  }

  return (
    <>
      <div className={s.page}>
        <AdminHeader
          title={
            <span>
              {t('adminTasks.title')}{' '}
              <span className={s.countBadge}>({total})</span>
            </span>
          }
        >
          <button 
            type="button"
            className={s.selectToggleBtn} 
            data-active={isSelectMode}
            onClick={handleSelectToggle}
          >
            {isSelectMode ? t('common.cancel') : t('adminTasks.select')}
          </button>
        </AdminHeader>

        <div className={s.filterBar}>
          <button type="button" className={s.filterBtn} data-active={filter === 'all'} onClick={() => setFilter('all')}>
            {t('adminTasks.filterAll')}
          </button>
          <button type="button" className={s.filterBtn} data-active={filter === 'pending'} onClick={() => setFilter('pending')}>
            {t('adminTasks.filterHealthy')}
          </button>
          <button type="button" className={s.filterBtn} data-active={filter === 'errored'} onClick={() => setFilter('errored')}>
            {t('adminTasks.filterErrored')}
          </button>
        </div>

        {loading && <div className={s.loading}>{t('common.loading')}</div>}

        {!loading && total === 0 && (
          <div className={s.empty}>{t('adminTasks.empty')}</div>
        )}

        {filteredTasks.length > 0 && (
          <>
            {isSelectMode && (
              <div className={s.selectionActions}>
                <button type="button" className={s.selectionBtn} onClick={toggleSelectAll}>
                  {allSelected ? t('adminTasks.unselectAll') : t('adminTasks.selectAll')}
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
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" stroke-linejoin="round">
                              <polyline points="20 6 9 17 4 12" />
                            </svg>
                          )}
                        </div>
                      </div>
                    )}
                    <div className={s.taskContent}>
                      <div className={s.taskPrimary}>
                        <div className={s.taskHeader}>
                          <span className={s.taskType}>{getTaskTypeLabel(task.task_type)}</span>
                          <span className={isDead ? s.badgeDead : s.badgePending}>
                            {isDead ? t('adminTasks.statusFailed') : t('adminTasks.statusPending')}
                          </span>
                        </div>
                        <div className={s.taskTitle}>{parseTitleFromPayload(task.payload)}</div>
                      </div>

                      <div className={s.taskDetails}>
                        <div className={s.taskMeta}>
                          {task.attempts}/{task.max_attempts} — day {task.day} · {formatRelativeTime(task.run_at)}
                        </div>
                        {task.last_error && (
                          <div className={s.taskError}>{task.last_error}</div>
                        )}
                      </div>

                      {isErrored && (
                        <div className={s.taskActions}>
                          {isDead && (
                            <button
                              type="button"
                              className={s.retryBtn}
                              onClick={(e) => { e.stopPropagation(); handleRetry(task.id); }}
                              disabled={acting === task.id || acting === -1}
                            >
                              {t('common.retry')}
                            </button>
                          )}
                          <button
                            type="button"
                            className={s.deleteBtn}
                            onClick={(e) => { e.stopPropagation(); openDeleteModal(task.id); }}
                            disabled={acting === task.id || acting === -1}
                          >
                            {t('common.delete')}
                          </button>
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </section>

            {hasMore && (
              <div className={s.loadMoreContainer}>
                <button 
                  type="button"
                  className={s.loadMoreBtn} 
                  onClick={() => setPage(p => p + 1)}
                  disabled={loading}
                >
                  {loading ? t('common.loading') : t('common.loadMore')}
                </button>
              </div>
            )}
          </>
        )}
      </div>

      {selectedIds.size > 0 && (
        <div className={s.actionBar}>
          <span className={s.actionText}>
            {t('adminTasks.selectedCount', { count: String(selectedIds.size) })}
          </span>
          <button 
            type="button"
            className={s.batchDeleteBtn}
            onClick={() => openDeleteModal('batch')}
            disabled={acting === -1}
          >
            {t('common.delete')}
          </button>
        </div>
      )}

      <ConfirmationDrawer
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onConfirm={confirmDelete}
        title={modalMode === 'batch'
          ? t('adminTasks.deleteBatchConfirm', { count: String(selectedIds.size) })
          : t('adminTasks.deleteSingleConfirm')}
        confirmText={t('common.delete')}
        cancelText={t('common.cancel')}
        isDangerous
      />
    </>
  )
}