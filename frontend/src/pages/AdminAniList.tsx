import { useState } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { PullToRefresh } from '../components/PullToRefresh'
import type { Settings } from '../types'
import s from './AdminAniList.module.css'

export function AdminAniList({ path }: { path?: string }) {
  const { data: settings, mutate: refetch } = useApi<Settings>('/settings')
  const [busy, setBusy] = useState(false)

  const handleConnect = () => {
    window.location.href = '/api/anilist/auth'
  }

  const handleDisconnect = async () => {
    if (busy) return
    setBusy(true)
    try {
      await apiFetch('/anilist/token', { method: 'DELETE' })
      refetch()
    } finally {
      setBusy(false)
    }
  }

  const connected = settings?.anilist_connected === true

  return (
    <PullToRefresh onRefresh={refetch}>
      <div className={s.page}>
        <div className={s.header}>
          <button type="button" onClick={() => history.back()} className={s.backBtn} aria-label="Back">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
            </svg>
          </button>
          <h1 className={s.title}>AniList</h1>
        </div>

        {!settings && <div className={s.loading}>Loading...</div>}

        {settings && (
          <div className={s.list}>
            <div className={s.item}>
              <div className={s.itemInfo}>
                <div className={s.itemLabel}>Connection</div>
                <div className={s.itemDesc}>
                  {connected ? 'Connected to AniList' : 'Not connected'}
                </div>
              </div>
              <span className={connected ? s.statusOn : s.statusOff}>
                {connected ? 'Connected' : 'Not connected'}
              </span>
            </div>

            <div className={s.actions}>
              {connected ? (
                <button
                  type="button"
                  className={s.dangerBtn}
                  onClick={handleDisconnect}
                  disabled={busy}
                >
                  {busy ? 'Disconnecting...' : 'Disconnect'}
                </button>
              ) : (
                <button
                  type="button"
                  className={s.primaryBtn}
                  onClick={handleConnect}
                  disabled={busy}
                >
                  Connect to AniList
                </button>
              )}
            </div>
          </div>
        )}
      </div>
    </PullToRefresh>
  )
}
