import { useEffect, useRef, useState } from 'preact/hooks'
import { route } from 'preact-router'
import { colors } from '../theme'
import { apiFetch } from '../api'

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
    <div style={{
      minHeight: '100vh',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      background: colors.bgPrimary,
      padding: '32px',
    }}>
      {/* Logo / Title */}
      <div style={{ marginBottom: '48px', textAlign: 'center' }}>
        <img
          src="/icon.png"
          alt="PlexTracker"
          style={{ width: '80px', height: '80px', margin: '0 auto 20px', display: 'block' }}
        />
        <div style={{ fontSize: '24px', fontWeight: 700, color: colors.textPrimary }}>PlexTracker</div>
        <div style={{ fontSize: '13px', color: colors.textSecondary, marginTop: '6px' }}>
          Track your media library
        </div>
      </div>

      {/* Google Sign-In */}
      <div ref={btnRef} style={{ minHeight: '44px' }} />

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
              setDevError('Invalid credentials')
            }
          }}
          style={{ marginTop: '24px', display: 'flex', flexDirection: 'column', gap: '8px', width: '320px' }}
        >
          <div style={{ fontSize: '11px', color: colors.textSecondary, textAlign: 'center' }}>Dev Login</div>
          <input
            type="text"
            placeholder="Username"
            value={username}
            onInput={(e) => setUsername((e.target as HTMLInputElement).value)}
            style={{ padding: '8px 12px', borderRadius: '8px', border: `1px solid ${colors.borderCard}`, background: colors.bgCard, color: colors.textPrimary, fontSize: '13px' }}
          />
          <input
            type="password"
            placeholder="Password"
            value={password}
            onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
            style={{ padding: '8px 12px', borderRadius: '8px', border: `1px solid ${colors.borderCard}`, background: colors.bgCard, color: colors.textPrimary, fontSize: '13px' }}
          />
          {devError && <div style={{ fontSize: '11px', color: '#e74c3c', textAlign: 'center' }}>{devError}</div>}
          <button
            type="submit"
            style={{ padding: '8px 12px', borderRadius: '8px', border: 'none', background: colors.accent, color: '#000', fontWeight: 600, fontSize: '13px', cursor: 'pointer' }}
          >
            Sign in
          </button>
        </form>
      )}

      <div style={{ fontSize: '11px', color: colors.textDimmed, marginTop: '24px', textAlign: 'center' }}>
        Sign in with your Google account to get started
      </div>
    </div>
  )
}
