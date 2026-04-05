import { useState, useEffect } from 'preact/hooks'
import { apiFetch } from '../api'
import s from './AdminNotifications.module.css'

interface NotifPrefs {
  notif_rating_prompt: boolean
  notif_dead_task: boolean
  notif_series_ended: boolean
}

const notifTypes = [
  {
    key: 'notif_rating_prompt' as const,
    label: 'Rappel de notation',
    description: 'Après un film ou une saison terminée',
  },
  {
    key: 'notif_dead_task' as const,
    label: 'Tâche échouée',
    description: 'Quand une tâche d\'enrichissement échoue définitivement',
  },
  {
    key: 'notif_series_ended' as const,
    label: 'Série terminée',
    description: 'Quand une série passe au statut terminée ou annulée',
  },
]

export function AdminNotifications({ path }: { path?: string }) {
  const [prefs, setPrefs] = useState<NotifPrefs | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    apiFetch<NotifPrefs>('/admin/notifications').then(setPrefs)
  }, [])

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
    <div className={s.page}>
      <div className={s.header}>
        <div onClick={() => history.back()} className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
        </div>
        <h1 className={s.title}>Notifications</h1>
      </div>

      {!prefs && <div className={s.loading}>Chargement...</div>}

      {prefs && (
        <div className={s.list}>
          {notifTypes.map((notif) => (
            <div key={notif.key} className={s.item}>
              <div className={s.itemInfo}>
                <div className={s.itemLabel}>{notif.label}</div>
                <div className={s.itemDesc}>{notif.description}</div>
              </div>
              <button
                className={prefs[notif.key] ? s.toggleOn : s.toggleOff}
                onClick={() => toggle(notif.key)}
                disabled={saving}
                aria-label={`${notif.label}: ${prefs[notif.key] ? 'activé' : 'désactivé'}`}
              >
                <span className={s.toggleKnob} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
