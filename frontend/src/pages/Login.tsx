import { useEffect, useRef, useState } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import s from './Login.module.css'

declare global {
  interface Window {
    google?: {
      accounts: {
        id: {
          initialize: (config: Record<string, unknown>) => void
          renderButton: (el: HTMLElement, config: Record<string, unknown>) => void
        }
      }
    }
  }
}

export function Login({ path }: { path?: string }) {
  const btnRef = useRef<HTMLDivElement>(null)
  const [devLogin, setDevLogin] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [devError, setDevError] = useState('')

  useEffect(() => {
    const init = async () => {
      const res = await fetch('/api/config')
      if (!res.ok) return
      const cfg = await res.json()
      if (!cfg) return
      if (cfg.dev_login) setDevLogin(true)
      const clientId = cfg.google_client_id
      if (!clientId || clientId === 'dev') return

      const script = document.createElement('script')
      script.src = 'https://accounts.google.com/gsi/client'
      script.async = true
      script.onload = () => {
        if (!window.google || !btnRef.current) return
        window.google.accounts.id.initialize({
          client_id: clientId,
          callback: handleCredentialResponse,
        })
        window.google.accounts.id.renderButton(btnRef.current, {
          theme: 'filled_black',
          size: 'large',
          width: '320',
          text: 'signin_with',
          shape: 'pill',
        })
      }
      document.head.appendChild(script)
    }
    init()
  }, [])

  const handleCredentialResponse = async (response: { credential: string }) => {
    await apiFetch('/auth/google', {
      method: 'POST',
      body: JSON.stringify({ credential: response.credential }),
    })
    route('/')
  }

  return (
    <div className={s.page}>
      {/* Logo / Title */}
      <div className={s.header}>
        <img src="/plextracker-logo.png" alt="PlexTracker" className={s.logo} />
        <div className={s.title}>PlexTracker</div>
        <div className={s.subtitle}>
          Suivez votre bibliothèque multimédia
        </div>
      </div>

      {/* Google Sign-In */}
      <div ref={btnRef} className={s.googleBtn} />

      {devLogin && (
        <form
          onSubmit={async (e) => {
            e.preventDefault()
            setDevError('')
            try {
              await apiFetch('/auth/dev', {
                method: 'POST',
                body: JSON.stringify({ username, password }),
              })
              route('/')
            } catch {
              setDevError('Identifiants invalides')
            }
          }}
          className={s.devForm}
        >
          <div className={s.devLabel}>Dev Login</div>
          <input
            type="text"
            name="username"
            id="username"
            autocomplete="username"
            placeholder="Username"
            value={username}
            onInput={(e) => setUsername((e.target as HTMLInputElement).value)}
            className={s.devInput}
          />
          <input
            type="password"
            name="password"
            id="password"
            autocomplete="current-password"
            placeholder="Password"
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            className={s.devInput}
          />
          {devError && <div className={s.devError}>{devError}</div>}
          <button type="submit" className={s.devSubmit}>
            Se connecter
          </button>
        </form>
      )}

      <div className={s.footer}>
        Connectez-vous avec votre compte Google pour commencer
      </div>
    </div>
  )
}
