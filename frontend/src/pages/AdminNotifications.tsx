import type { JSX } from 'preact'
import { useState, useEffect } from 'preact/hooks'
import { useApi } from '../hooks/useApi'
import { apiFetch } from '../api'
import { PullToRefresh } from '../components/PullToRefresh'
import { AdminHeader } from '../components/AdminHeader'
import s from './AdminNotifications.module.css'

interface NotifPrefs {
  notif_rating_prompt: boolean
  notif_dead_task: boolean
  notif_series_ended: boolean
}

const notifTypes = [
  {
    key: 'notif_rating_prompt' as const,
    label: 'Rating reminder',
    description: 'After a movie or completed season',
  },
  {
    key: 'notif_dead_task' as const,
    label: 'Failed task',
    description: 'When an enrichment task permanently fails',
  },
  {
    key: 'notif_series_ended' as const,
    label: 'Series ended',
    description: 'When a series status changes to ended or cancelled',
  },
]

export function AdminNotifications({ path }: { path?: string }): JSX.Element {
  const { data: fetchedPrefs, mutate: refetch } = useApi<NotifPrefs>('/admin/notifications')
  const [prefs, setPrefs] = useState<NotifPrefs | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (fetchedPrefs && !prefs) setPrefs(fetchedPrefs)
  }, [fetchedPrefs])

  const toggle = async (key: keyof NotifPrefs) => {
    if (!prefs || saving) return
    const updated = { ...prefs, [key]: !prefs[key] }
    setPrefs(updated)
    setSaving(true)
    try {
      await apiFetch('/admin/notifications', {
        method: 'PUT',
        body: JSON.stringify({ [key]: updated[key] }),
      })
    } finally {
      setSaving(false)
    }
  }

  return (
    <PullToRefresh onRefresh={refetch}>
    <div className={s.page}>
      <AdminHeader title="Notifications" />

      {!prefs && <div className={s.loading}>Loading...</div>}

      {prefs && (
        <div className={s.list}>
          {notifTypes.map((notif) => (
            <div key={notif.key} className={s.item}>
              <div className={s.itemInfo}>
                <div className={s.itemLabel}>{notif.label}</div>
                <div className={s.itemDesc}>{notif.description}</div>
              </div>
              <button
                type="button"
                role="switch"
                aria-checked={Boolean(prefs[notif.key])}
                className={prefs[notif.key] ? s.toggleOn : s.toggleOff}
                onClick={() => void toggle(notif.key)}
                disabled={saving}
                aria-label={`${notif.label}: ${prefs[notif.key] ? 'enabled' : 'disabled'}`}
              >
                <span className={s.toggleKnob} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
    </PullToRefresh>
  )
}
