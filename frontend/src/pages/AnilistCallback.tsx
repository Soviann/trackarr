// AniList OAuth implicit-grant landing page.
// The redirect URI `http://localhost:8080/anilist/callback` is registered on the AniList dev OAuth client.
import { useEffect, useState } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import { routeTo } from '../routes'
import { useTranslation } from '../i18n'
import s from './AnilistCallback.module.css'

type Status = 'pending' | 'error'

export function AnilistCallback({ path }: { path?: string }) {
  void path
  const { t } = useTranslation()
  const [status, setStatus] = useState<Status>('pending')

  useEffect(() => {
    const token = new URLSearchParams(window.location.hash.slice(1)).get('access_token')
    // The bearer token must never persist in the URL: clear it immediately,
    // whether exchange succeeds or fails (otherwise it leaks via Referer or browser history).
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
        route(routeTo.adminAniList(), true)
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
          <p className={s.message}>{t('anilistCallback.connecting')}</p>
        ) : (
          <>
            <p className={s.title}>{t('anilistCallback.errorTitle')}</p>
            <p className={s.subtitle}>{t('anilistCallback.errorDesc')}</p>
            <a href={routeTo.adminAniList()} className={s.link}>{t('anilistCallback.tryAgain')}</a>
          </>
        )}
      </div>
    </div>
  )
}
