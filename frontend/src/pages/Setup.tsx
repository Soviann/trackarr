import { useEffect, useState } from 'preact/hooks'
import { route } from 'preact-router'
import { apiFetch } from '../api'
import { routeTo } from '../routes'
import s from './Login.module.css'

export function Setup({ path }: { path?: string }) {
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [authMode, setAuthMode] = useState<'hybrid' | 'password' | 'google'>('hybrid')
  const [hasGoogle, setHasGoogle] = useState(false)
  const [error, setError] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [recoveryKey, setRecoveryKey] = useState<string | null>(null)
  const [keySaved, setKeySaved] = useState(false)
  const [copiedKey, setCopiedKey] = useState(false)

  useEffect(() => {
    fetch('/api/config')
      .then((r) => r.json())
      .then((cfg) => {
        if (!cfg.setup_required) {
          route(routeTo.login())
        }
        if (cfg.google_client_id) {
          setHasGoogle(true)
        }
      })
      .catch((err) => console.error('Failed to load setup config:', err))
  }, [])

  const handleSubmit = async (e: Event) => {
    e.preventDefault()
    setError('')
    if (password.length < 8) {
      setError('Password must be at least 8 characters')
      return
    }
    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    setSubmitting(true)
    try {
      const res = await apiFetch<{ recovery_key: string }>('/auth/setup', {
        method: 'POST',
        body: JSON.stringify({
          username: username.trim() || 'admin',
          password,
          auth_mode: authMode,
        }),
      })
      setRecoveryKey(res.recovery_key)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Setup failed')
    } finally {
      setSubmitting(false)
    }
  }

  const handleCopyKey = () => {
    if (recoveryKey) {
      navigator.clipboard.writeText(recoveryKey)
      setCopiedKey(true)
      setKeySaved(true)
      setTimeout(() => setCopiedKey(false), 3000)
    }
  }

  return (
    <div className={s.page}>
      <div className={s.card}>
        <div className={s.header}>
          <img src="/favicon.svg" alt="Trackarr" className={s.logo} />
          <div className={s.title}>Welcome to Trackarr</div>
          <div className={s.subtitle}>First-Time Setup Wizard</div>
        </div>

        {recoveryKey ? (
          <div className={s.keyAlert}>
            <div className={s.keyHeader}>🔑 Save Your Emergency Recovery Key</div>
            <div className={s.keyDescription}>
              This key is your <strong>direct emergency method</strong> to reset your password from the browser without an email server (you can also use the server command <code>trackarr reset-password</code>).
              It is stored as an irreversible cryptographic hash (bcrypt): it is <strong>strictly impossible to recover or decrypt</strong>, even with database access. It will <strong>never be displayed again</strong>.
            </div>
            <div className={s.keyCodeBox}>{recoveryKey}</div>

            <button type="button" onClick={handleCopyKey} className={s.secondaryBtn}>
              {copiedKey ? '✅ Key copied to clipboard!' : 'Copy Recovery Key'}
            </button>

            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: 'var(--font-size-xs)', cursor: 'pointer', marginTop: '8px' }}>
              <input
                type="checkbox"
                checked={keySaved}
                onChange={(e) => setKeySaved((e.target as HTMLInputElement).checked)}
              />
              <span>I have safely written down and saved this recovery key</span>
            </label>

            <button
              type="button"
              disabled={!keySaved}
              onClick={() => route(routeTo.home())}
              className={s.primaryBtn}
              style={{ marginTop: '8px' }}
            >
              Complete Setup and Open Trackarr
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className={s.form}>
            {error && <div className={s.errorAlert}>{error}</div>}

            <div className={s.inputGroup}>
              <label htmlFor="setup-user" className={s.label}>Administrator Username</label>
              <input
                id="setup-user"
                type="text"
                value={username}
                onInput={(e) => setUsername((e.target as HTMLInputElement).value)}
                className={s.input}
                required
              />
            </div>

            <div className={s.inputGroup}>
              <label htmlFor="setup-pass" className={s.label}>Password (min. 8 characters)</label>
              <input
                id="setup-pass"
                type="password"
                value={password}
                onInput={(e) => setPassword((e.target as HTMLInputElement).value)}
                className={s.input}
                required
              />
            </div>

            <div className={s.inputGroup}>
              <label htmlFor="setup-confirm-pass" className={s.label}>Confirm Password</label>
              <input
                id="setup-confirm-pass"
                type="password"
                value={confirmPassword}
                onInput={(e) => setConfirmPassword((e.target as HTMLInputElement).value)}
                className={s.input}
                required
              />
            </div>

            {hasGoogle && (
              <div className={s.inputGroup}>
                <label htmlFor="setup-mode" className={s.label}>Authentication Mode</label>
                <select
                  id="setup-mode"
                  value={authMode}
                  onChange={(e) => setAuthMode((e.target as HTMLSelectElement).value as any)}
                  className={s.input}
                >
                  <option value="hybrid">Hybrid (Local Credentials + Google OAuth)</option>
                  <option value="password">Local Credentials Only (without Google)</option>
                  <option value="google">Google OAuth Only</option>
                </select>
              </div>
            )}

            <button type="submit" disabled={submitting} className={s.primaryBtn} style={{ marginTop: '12px' }}>
              {submitting ? 'Setting up...' : 'Create Account & Generate Key'}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
