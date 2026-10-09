import type { JSX } from 'preact'
import { useState } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { PullToRefresh } from '../components/PullToRefresh'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import { AdminHeader } from '../components/AdminHeader'
import { useTranslation } from '../i18n'
import type { Settings } from '../types'
import s from './AdminAniList.module.css'

export function AdminAniList({ path }: { path?: string }): JSX.Element {
  const { t } = useTranslation()
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
        <AdminHeader title={t('adminAniList.title')} />

        {settings?.anilist_token_invalid === true && (
          <div className={s.reconnectBanner} role="alert">
            <div>
              <strong>{t('adminAniList.expiredTitle')}</strong>
              <p>{t('adminAniList.expiredDesc')}</p>
            </div>
            <a className={s.reconnectButton} href="/api/anilist/auth">{t('adminAniList.reconnect')}</a>
          </div>
        )}

        {!settings && <div className={s.loading}>{t('common.loading')}</div>}

        {settings && (
          <div className={s.list}>
            <div className={s.item}>
              <div className={s.itemInfo}>
                <div className={s.itemLabel}>{t('adminAniList.connectionLabel')}</div>
                <div className={s.itemDesc}>
                  {connected ? t('adminAniList.connectedDesc') : t('adminAniList.notConnectedDesc')}
                </div>
              </div>
              <span className={connected ? s.statusOn : s.statusOff}>
                {connected ? t('adminAniList.connected') : t('adminAniList.notConnected')}
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
                  {busy ? t('adminAniList.disconnecting') : t('adminAniList.disconnect')}
                </button>
              ) : (
                <button
                  type="button"
                  className={s.primaryBtn}
                  onClick={handleConnect}
                  disabled={busy}
                >
                  {t('adminAniList.connect')}
                </button>
              )}
            </div>
          </div>
        )}

        <ConfirmationDrawer
          open={confirmDisconnectOpen}
          onClose={() => setConfirmDisconnectOpen(false)}
          onConfirm={handleDisconnect}
          title={t('adminAniList.disconnectModalTitle')}
          description={t('adminAniList.disconnectModalDesc')}
          confirmText={t('adminAniList.disconnect')}
          cancelText={t('common.cancel')}
          isDangerous
        />
      </div>
    </PullToRefresh>
  )
}
