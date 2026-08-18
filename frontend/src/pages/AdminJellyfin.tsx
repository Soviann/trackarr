import { useApi } from '../hooks/useApi'
import { colors } from '../theme'
import { PullToRefresh } from '../components/PullToRefresh'
import type { Settings } from '../types'
import s from './AdminJellyfin.module.css'

export function AdminJellyfin({ path }: { path?: string }) {
  const { data: settings, mutate: refetch } = useApi<Settings>('/settings')

  const configured = settings?.jellyfin_configured === true
  const lastScrobble = settings?.jellyfin_last_scrobble_at
    ? new Date(settings.jellyfin_last_scrobble_at).toLocaleString()
    : 'None recorded'

  return (
    <PullToRefresh onRefresh={refetch}>
      <div className={s.page}>
        <div className={s.header}>
          <button type="button" onClick={() => history.back()} className={s.backBtn} aria-label="Back">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={colors.ink} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
            </svg>
          </button>
          <h1 className={s.title}>Jellyfin</h1>
        </div>

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
