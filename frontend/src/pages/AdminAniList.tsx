import type { JSX } from 'preact'
import { useState } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { PullToRefresh } from '../components/PullToRefresh'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import { AdminHeader } from '../components/AdminHeader'
import type { Settings } from '../types'
import s from './AdminAniList.module.css'

export function AdminAniList({ path }: { path?: string }): JSX.Element {
  const { data: settings, mutate: refetch } = useApi<Settings>('/settings')
  const [busy, setBusy] = useState(false)
  const [confirmDisconnectOpen, setConfirmDisconnectOpen] = useState(false)

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
        <AdminHeader title="AniList" />

        {settings?.anilist_token_invalid === true && (
          <div className={s.reconnectBanner} role="alert">
            <div>
              <strong>AniList connection expired.</strong>
              <p>Rating &amp; status sync is paused until you reconnect.</p>
            </div>
            <a className={s.reconnectButton} href="/api/anilist/auth">Reconnect</a>
          </div>
        )}

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
                  onClick={() => setConfirmDisconnectOpen(true)}
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

        <ConfirmationDrawer
          open={confirmDisconnectOpen}
          onClose={() => setConfirmDisconnectOpen(false)}
          onConfirm={handleDisconnect}
          title="Disconnect AniList?"
          description="This will remove your AniList authentication token. Trackarr will stop scrobbling anime watch status to AniList until reconnected."
          confirmText="Disconnect"
          cancelText="Cancel"
          isDangerous
        />
      </div>
    </PullToRefresh>
  )
}
