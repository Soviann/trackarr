import type { JSX } from 'preact'
import { useApi } from '../hooks/useApi'
import { PullToRefresh } from '../components/PullToRefresh'
import { AdminHeader } from '../components/AdminHeader'
import { formatDateTime24h } from '../utils'
import type { Settings } from '../types'
import s from './AdminJellyfin.module.css'

export function AdminJellyfin({ path }: { path?: string }): JSX.Element {
  const { data: settings, mutate: refetch } = useApi<Settings>('/settings')

  const configured = settings?.jellyfin_configured === true
  const lastScrobble = settings?.jellyfin_last_scrobble_at
    ? formatDateTime24h(settings.jellyfin_last_scrobble_at)
    : 'None recorded'

  return (
    <PullToRefresh onRefresh={refetch}>
      <div className={s.page}>
        <AdminHeader title="Jellyfin" />

        {!settings && <div className={s.loading}>Loading...</div>}

        {settings && (
          <div className={s.list}>
            <div className={s.item}>
              <div className={s.itemInfo}>
                <div className={s.itemLabel}>Webhook status</div>
                <div className={s.itemDesc}>
                  {configured ? 'JELLYFIN_WEBHOOK_SECRET is configured' : 'JELLYFIN_WEBHOOK_SECRET is missing'}
                </div>
              </div>
              <span className={configured ? s.statusOn : s.statusOff}>
                {configured ? 'Configured' : 'Missing secret'}
              </span>
            </div>

            <div className={s.item}>
              <div className={s.itemInfo}>
                <div className={s.itemLabel}>Last scrobble received</div>
                <div className={s.itemDesc}>
                  Latest playback completion recorded from Jellyfin
                </div>
              </div>
              <span className={settings.jellyfin_last_scrobble_at ? s.statusOn : s.statusOff}>
                {lastScrobble}
              </span>
            </div>

            <div className={s.helpCard}>
              <h2 className={s.helpTitle}>Webhook Configuration in Jellyfin</h2>
              <p className={s.helpText}>
                In your Jellyfin server, navigate to <strong>Dashboard &rarr; Plugins &rarr; Webhook &rarr; Add Generic Destination</strong>:
              </p>
              <p className={s.helpText}>
                <strong>Webhook URL:</strong> <code>https://&lt;your-domain&gt;/api/webhook/jellyfin/&lt;secret&gt;</code><br />
                <strong>Notification Type:</strong> Playback Stop<br />
                <strong>Item Type:</strong> Movies &amp; Episodes
              </p>
            </div>
          </div>
        )}
      </div>
    </PullToRefresh>
  )
}
