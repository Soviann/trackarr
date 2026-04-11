import { useApi } from '../hooks/useApi'
import type { EpisodeHistory } from '../types'
import s from './TitleHistory.module.css'

interface Props {
  titleId: number
  onClose: () => void
}

export function TitleHistory({ titleId, onClose }: Props) {
  const { data, loading } = useApi<EpisodeHistory[]>(`/titles/${titleId}/history`)

  return (
    <div className={s.container}>
      <div className={s.header}>
        <button className={s.backBtn} onClick={onClose} aria-label="Back">←</button>
        <span className={s.title}>History</span>
      </div>
      {loading && <div className={s.loading}>Loading…</div>}
      {data?.map((ep, i) => (
        <div key={i} className={s.row}>
          <div className={s.info}>
            <span className={s.epLabel}>
              {ep.episode_number != null
                ? `S${ep.season_number} E${ep.episode_number}${ep.episode_name ? ` — ${ep.episode_name}` : ''}`
                : 'Movie'}
            </span>
            <span className={s.date}>
              {new Date(ep.last_watched_at).toLocaleDateString('en-US', {
                day: 'numeric', month: 'short', year: 'numeric'
              })}
            </span>
          </div>
          {ep.watch_count > 1 && (
            <span className={s.rewatchBadge}>×{ep.watch_count}</span>
          )}
        </div>
      ))}
      {!loading && data?.length === 0 && (
        <div className={s.loading}>No watches recorded.</div>
      )}
    </div>
  )
}
