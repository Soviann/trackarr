import { useEffect, useRef, useState } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import { routeTo } from '../routes'
import { useTranslation } from '../i18n'
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

interface PublicConfig {
  google_client_id?: string
  google_auth_enabled: boolean
  password_auth_enabled: boolean
  auth_mode: 'google' | 'password' | 'hybrid'
  setup_required: boolean
  vapid_public_key?: string
}

export function Login({ path }: { path?: string }) {
  const { t } = useTranslation()
  const btnRef = useRef<HTMLDivElement>(null)
  const [config, setConfig] = useState<PublicConfig | null>(null)
  const [loading, setLoading] = useState(true)

  // Local login state
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [loginError, setLoginError] = useState('')
  const [submitting, setSubmitting] = useState(false)

  // Recovery mode state
  const [isRecovering, setIsRecovering] = useState(false)
  const [recoveryKey, setRecoveryKey] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [recoveryError, setRecoveryError] = useState('')
  const [newRecoveryKey, setNewRecoveryKey] = useState<string | null>(null)
  const [copiedKey, setCopiedKey] = useState(false)

  useEffect(() => {
    const init = async () => {
      try {
        const res = await fetch('/api/config')
        if (!res.ok) return
        const cfg: PublicConfig = await res.json()
        setConfig(cfg)

        if (cfg.setup_required) {
          route(routeTo.setup())
          return
        }

        if (cfg.google_auth_enabled && cfg.google_client_id && cfg.google_client_id !== 'dev') {
          const script = document.createElement('script')
          script.src = 'https://accounts.google.com/gsi/client'
          script.async = true
          script.onload = () => {
            if (!window.google || !btnRef.current) return
            window.google.accounts.id.initialize({
              client_id: cfg.google_client_id,
              callback: handleGoogleResponse,
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
      } catch (err) {
        console.error('Failed to initialize login:', err)
      } finally {
        setLoading(false)
      }
    }
    init()
  }, [])

  const handleGoogleResponse = async (response: { credential: string }) => {
    try {
      await apiFetch('/auth/google', {
        method: 'POST',
        body: JSON.stringify({ credential: response.credential }),
      })
      route(routeTo.home())
    } catch (err: unknown) {
      setLoginError(err instanceof Error ? err.message : 'Google authentication failed')
    }
  }

  const handleLocalLogin = async (e: Event) => {
    e.preventDefault()
    setLoginError('')
    setSubmitting(true)
    try {
      await apiFetch('/auth/login', {
        method: 'POST',
        body: JSON.stringify({ username, password }),
      })
      route(routeTo.home())
    } catch (err: unknown) {
      setLoginError(err instanceof Error ? err.message : 'Invalid credentials')
    } finally {
      setSubmitting(false)
    }
  }

  const handleRecover = async (e: Event) => {
    e.preventDefault()
    setRecoveryError('')
    if (newPassword.length < 8) {
      setRecoveryError('Password must be at least 8 characters')
      return
    }
    if (newPassword !== confirmPassword) {
      setRecoveryError('Passwords do not match')
      return
    }

    setSubmitting(true)
    try {
      const res = await apiFetch<{ new_recovery_key: string }>('/auth/recover', {
        method: 'POST',
        body: JSON.stringify({
          recovery_key: recoveryKey,
          new_password: newPassword,
        }),
      })
      setNewRecoveryKey(res.new_recovery_key)
    } catch (err: unknown) {
      setRecoveryError(err instanceof Error ? err.message : 'Invalid recovery key')
    } finally {
      setSubmitting(false)
    }
  }

  const handleCopyKey = () => {
    if (newRecoveryKey) {
      navigator.clipboard.writeText(newRecoveryKey)
      setCopiedKey(true)
      setTimeout(() => setCopiedKey(false), 3000)
    }
  }

  if (loading || !config) {
    return <div className={s.page} />
  }

  const showPasswordForm = !isRecovering && config.password_auth_enabled
  const showGoogleAuth = !isRecovering && config.google_auth_enabled
  const showDivider = showPasswordForm && showGoogleAuth

  return (
    <div className={s.page}>
      <div className={s.card}>
        <div className={s.header}>
          <img src="/favicon.svg" alt="Trackarr" className={s.logo} />
          <div className={s.title}>Trackarr</div>
          <div className={s.subtitle}>
            {isRecovering ? t('auth.emergencyRecovery') : t('auth.loginSubtitle')}
          </div>
        </div>

        {/* RECOVERY RESULT VIEW */}
        {newRecoveryKey ? (
          <div className={s.keyAlert}>
            <div className={s.keyHeader}>⚠️ {t('auth.newPasswordActivated')}</div>
            <div className={s.keyDescription}>
              {t('auth.newPasswordDesc')}
            </div>
            <div className={s.keyCodeBox}>{newRecoveryKey}</div>
            <div className={s.copyRow}>
              <button type="button" onClick={handleCopyKey} className={s.secondaryBtn} style={{ flex: 1 }}>
                {copiedKey ? `✅ ${t('auth.keyCopied')}` : t('auth.copyKey')}
              </button>
              <button
                type="button"
                onClick={() => route(routeTo.home())}
                className={s.primaryBtn}
                style={{ flex: 1 }}
              >
                {t('auth.openTrackarr')}
              </button>
            </div>
          </div>
        ) : isRecovering ? (
          /* RECOVERY FORM */
          <form onSubmit={handleRecover} className={s.form}>
            {recoveryError && <div className={s.errorAlert}>{recoveryError}</div>}

            <div className={s.inputGroup}>
              <label htmlFor="recovery-key" className={s.label}>{t('auth.recoveryKey')}</label>
              <input
                id="recovery-key"
                type="text"
                placeholder="TRCK-XXXX-XXXX-XXXX"
                value={recoveryKey}
                onInput={(e) => setRecoveryKey((e.target as HTMLInputElement).value)}
                className={s.input}
                autoFocus
                required
              />
            </div>

            <div className={s.inputGroup}>
              <label htmlFor="new-pass" className={s.label}>{t('auth.newPassword')}</label>
              <input
                id="new-pass"
                type="password"
                autocomplete="new-password"
                placeholder="Minimum 8 characters"
                value={newPassword}
                onInput={(e) => setNewPassword((e.target as HTMLInputElement).value)}
                className={s.input}
                required
              />
            </div>

            <div className={s.inputGroup}>
              <label htmlFor="confirm-pass" className={s.label}>{t('auth.confirmPassword')}</label>
              <input
                id="confirm-pass"
                type="password"
                autocomplete="new-password"
                placeholder="Repeat password"
                value={confirmPassword}
                onInput={(e) => setConfirmPassword((e.target as HTMLInputElement).value)}
                className={s.input}
                required
              />
            </div>

            <button type="submit" disabled={submitting} className={s.primaryBtn}>
              {submitting ? t('auth.resetting') : t('auth.resetPassword')}
            </button>

            <button
              type="button"
              onClick={() => {
                setIsRecovering(false)
                setRecoveryError('')
              }}
              className={s.linkBtn}
            >
              {t('common.back')}
            </button>
          </form>
        ) : (
          /* REGULAR LOGIN VIEW */
          <div className={s.form}>
            {loginError && <div className={s.errorAlert}>{loginError}</div>}

            {/* Local Password Form */}
            {showPasswordForm && (
              <form onSubmit={handleLocalLogin} className={s.form}>
                <div className={s.inputGroup}>
                  <label htmlFor="login-username" className={s.label}>{t('auth.username')}</label>
                  <input
                    id="login-username"
                    type="text"
                    autocomplete="username"
                    value={username}
                    onInput={(e) => setUsername((e.target as HTMLInputElement).value)}
                    className={s.input}
                    required
                  />
                </div>

                <div className={s.inputGroup}>
                  <label htmlFor="login-password" className={s.label}>{t('auth.password')}</label>
                  <input
                    id="login-password"
                    type="password"
                    autocomplete="current-password"
                    value={password}
                    onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
                    className={s.input}
                    required
                  />
                </div>

                <button type="submit" disabled={submitting} className={s.primaryBtn}>
                  {submitting ? t('auth.signingIn') : t('auth.signIn')}
                </button>

                <button
                  type="button"
                  onClick={() => {
                    setIsRecovering(true)
                    setLoginError('')
                  }}
                  className={s.linkBtn}
                >
                  {t('auth.forgotPassword')}
                </button>
              </form>
            )}

            {/* Divider when both are shown */}
            {showDivider && <div className={s.divider}>{t('auth.or')}</div>}

            {/* Google OAuth Button Container */}
            {showGoogleAuth && (
              <div className={s.googleBtnContainer}>
                <div ref={btnRef} />
              </div>
            )}
          </div>
        )}
      </div>

      <div className={s.footer}>
        {config.auth_mode === 'google'
          ? 'Sign in with your authorized Google account'
          : 'Trackarr — Self-hosted personal media tracker'}
      </div>
    </div>
  )
}
