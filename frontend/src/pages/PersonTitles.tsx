import { useApi } from '../hooks/useApi'
import type { PaginatedResponse } from '../types'
import { TitleCard } from '../components/TitleCard'
import { ErrorBanner } from '../components/ErrorBanner'
import s from './PersonTitles.module.css'

import { useScrollRestoration } from '../hooks/useScrollRestoration'

export function PersonTitles({ name }: { path?: string; name?: string }) {
  const { data, error, loading, mutate } = useApi<PaginatedResponse>(
    name ? `/titles?person=${encodeURIComponent(name)}&limit=200` : null,
  )
  useScrollRestoration(`person-${name ?? ''}`, !loading)

  const titles = data?.titles ?? []

  return (
    <div className={s.page}>
      <div className={s.header}>
        <button type="button" className={s.backBtn} onClick={() => history.back()} aria-label="Back">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="15 18 9 12 15 6" />
          </svg>
        </button>
        <div className={s.personName}>{name}</div>
      </div>

      {error && <ErrorBanner message={error} onRetry={mutate} />}

      {loading && <div className={s.centered}>Loading…</div>}

      {!loading && !error && titles.length === 0 && (
        <div className={s.centered}>No titles found.</div>
      )}

      {titles.length > 0 && (
        <div className={s.list}>
          {titles.map((t) => (
            <TitleCard key={t.id} title={t} onUpdate={mutate} showSortCaption={false} />
          ))}
        </div>
      )}
    </div>
  )
}
