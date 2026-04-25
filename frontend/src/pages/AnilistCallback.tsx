// AniList OAuth implicit-grant landing page.
// The redirect URI `http://localhost:8080/anilist/callback` is registered on the AniList dev OAuth client.
import { useEffect, useState } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import s from './AnilistCallback.module.css'

type Status = 'pending' | 'error'

export function AnilistCallback({ path }: { path?: string }) {
  void path
  const [status, setStatus] = useState<Status>('pending')

  useEffect(() => {
    const token = new URLSearchParams(window.location.hash.slice(1)).get('access_token')
    // Le token bearer ne doit jamais persister dans l'URL : on l'efface immédiatement,
    // que l'échange réussisse ou non (sinon il fuirait via Referer ou l'historique).
    if (window.location.hash) {
      history.replaceState(null, '', window.location.pathname)
    }
    if (!token) {
      setStatus('error')
      return
    }

    let cancelled = false
    apiFetch('/anilist/token', {
      method: 'POST',
      body: JSON.stringify({ token }),
    })
      .then(() => {
        if (cancelled) return
        route('/admin/anilist', true)
      })
      .catch((err) => {
        if (cancelled) return
        console.error('AniList token exchange failed:', err)
        setStatus('error')
      })

    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className={s.page}>
      <div className={s.card}>
        {status === 'pending' ? (
          <p className={s.message}>Connecting to AniList...</p>
        ) : (
          <>
            <p className={s.title}>Couldn't connect to AniList</p>
            <p className={s.subtitle}>The access token is missing or was rejected.</p>
            <a href="/admin/anilist" className={s.link}>Try again</a>
          </>
        )}
      </div>
    </div>
  )
}
