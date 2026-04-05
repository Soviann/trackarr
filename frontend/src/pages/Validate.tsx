import { useState } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title, TitleStatus } from '../types'
import { useApi } from '../hooks/useApi'
import { getName } from '../utils'
import { StatusBadge } from '../components/StatusBadge'
import { apiFetch } from '../api'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import clsx from 'clsx'
import s from './Validate.module.css'

export function Validate({ path }: { path?: string }) {
  const params = new URLSearchParams(window.location.search)
  const query = params.get('q') ?? ''
  const searchPath = query ? `/titles?search=${encodeURIComponent(query)}` : null
  const { data: results, loading } = useApi<Title[]>(searchPath)
  const [adding, setAdding] = useState(false)
  const [selectedStatus, setSelectedStatus] = useState<TitleStatus>('plan_to_watch')

  const handleAdd = async () => {
    if (adding) return
    setAdding(true)
    try {
      const created = await apiFetch<Title>('/titles', {
        method: 'POST',
        body: JSON.stringify({
          type: 'series',
          year: new Date().getFullYear(),
          status: selectedStatus,
          match_status: 'unconfirmed',
          names: [{ name: query, language: 'en', is_primary: true }],
        }),
      })
      route(`/title/${created.id}`)
    } finally {
      setAdding(false)
    }
  }

  return (
    <div className={s.page}>
      {/* Header */}
      <div className={s.header}>
        <div onClick={() => history.back()} className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
        </div>
        <div className={s.headerTitle}>
          Validating: {query}
        </div>
      </div>

      {loading && (
        <div className={s.loading}>
          <div className={s.spinner} />
          Matching...
        </div>
      )}

      {/* Existing results */}
      {results && results.length > 0 && (
        <div className={s.resultsSection}>
          <div className={s.sectionLabel}>
            Already in library
          </div>
          {results.map((t) => (
            <div
              key={t.id}
              onClick={() => route(`/title/${t.id}`)}
              className={s.resultCard}
            >
              <div
                className={s.resultCover}
                style={{ background: coverBackground(t.cover_url, t.type) }}
              >
                {!t.cover_url && <CoverPlaceholder type={t.type} iconSize="18px" />}
              </div>
              <div className={s.resultInfo}>
                <div className={s.resultNameRow}>
                  <span className={s.resultName}>{getName(t)}</span>
                  <StatusBadge status={t.status} />
                </div>
                <div className={s.resultMeta}>
                  {t.type} · {t.year}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Add new */}
      {results && (
        <div className={s.addCard}>
          <div className={s.addCardTitle}>
            Add as new title
          </div>

          {/* Status picker */}
          <div className={s.statusPicker}>
            {(['watching', 'plan_to_watch', 'completed'] as TitleStatus[]).map((status) => (
              <button
                key={status}
                onClick={() => setSelectedStatus(status)}
                className={clsx(s.statusOption, selectedStatus === status && s.statusOptionSelected)}
              >
                {status === 'plan_to_watch' ? 'Plan to watch' : status.charAt(0).toUpperCase() + status.slice(1)}
              </button>
            ))}
          </div>

          <button
            onClick={handleAdd}
            disabled={adding}
            className={s.addBtn}
          >
            <span className={s.addBtnText}>
              {adding ? 'Adding...' : 'Add to library'}
            </span>
          </button>
        </div>
      )}
    </div>
  )
}
