import { useEffect, useRef } from 'preact/hooks'
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

  useEffect(() => {
    const script = document.createElement('script')
    script.src = 'https://accounts.google.com/gsi/client'
    script.async = true
    script.onload = () => {
      if (!window.google || !btnRef.current) return
      window.google.accounts.id.initialize({
        client_id: (window as Record<string, unknown>).__GOOGLE_CLIENT_ID__ ?? '',
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
    return () => { script.remove() }
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
        <div style={{
          width: '64px', height: '64px', borderRadius: '16px',
          background: `linear-gradient(135deg, ${colors.accentAmber}, ${colors.accentCoral})`,
          display: 'flex', alignItems: 'center', justifyContent: 'center',
          margin: '0 auto 20px',
        }}>
          <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="2" y="3" width="20" height="14" rx="2" ry="2" />
            <line x1="8" y1="21" x2="16" y2="21" />
            <line x1="12" y1="17" x2="12" y2="21" />
          </svg>
        </div>
        <div style={{ fontSize: '24px', fontWeight: 700, color: colors.textPrimary }}>PlexTracker</div>
        <div style={{ fontSize: '13px', color: colors.textSecondary, marginTop: '6px' }}>
          Track your media library
        </div>
      </div>

      {/* Google Sign-In */}
      <div ref={btnRef} style={{ minHeight: '44px' }} />

      <div style={{ fontSize: '11px', color: colors.textDimmed, marginTop: '24px', textAlign: 'center' }}>
        Sign in with your Google account to get started
      </div>
    </div>
  )
}
