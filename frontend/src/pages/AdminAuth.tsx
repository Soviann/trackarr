import { useEffect, useState } from 'preact/hooks'
import { apiFetch } from '../api'
import { colors } from '../theme'
import s from './AdminAuth.module.css'

interface AuthSettingsResponse {
  auth_mode: 'google' | 'password' | 'hybrid'
  has_password: boolean
  has_google: boolean
  google_email?: string
  username: string
}

export function AdminAuth({ path }: { path?: string }) {
  const [settings, setSettings] = useState<AuthSettingsResponse | null>(null)
  const [loading, setLoading] = useState(true)

  // Auth Mode Form
  const [authMode, setAuthMode] = useState<'google' | 'password' | 'hybrid'>('hybrid')
  const [username, setUsername] = useState('admin')
  const [modeSuccess, setModeSuccess] = useState('')
  const [modeError, setModeError] = useState('')
  const [savingMode, setSavingMode] = useState(false)

  // Password Change Form
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordSuccess, setPasswordSuccess] = useState('')
  const [passwordError, setPasswordError] = useState('')
  const [savingPassword, setSavingPassword] = useState(false)
  const [newKeyAfterPasswordChange, setNewKeyAfterPasswordChange] = useState<string | null>(null)

  // Recovery Key Regeneration
  const [newRegeneratedKey, setNewRegeneratedKey] = useState<string | null>(null)
  const [regeneratingKey, setRegeneratingKey] = useState(false)
  const [keyError, setKeyError] = useState('')

  const fetchSettings = async () => {
    try {
      const data = await apiFetch<AuthSettingsResponse>('/admin/auth-settings')
      setSettings(data)
      setAuthMode(data.auth_mode)
      setUsername(data.username || 'admin')
    } catch (err) {
      console.error('Failed to load auth settings:', err)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchSettings()
  }, [])

  const handleSaveMode = async (e: Event) => {
    e.preventDefault()
    setModeSuccess('')
    setModeError('')
    setSavingMode(true)
    try {
      await apiFetch('/admin/auth-settings', {
        method: 'PUT',
        body: JSON.stringify({
          auth_mode: authMode,
          username: username.trim() || 'admin',
        }),
      })
      setModeSuccess('Authentication mode updated!')
      fetchSettings()
    } catch (err: unknown) {
      setModeError(err instanceof Error ? err.message : 'Failed to update authentication mode')
    } finally {
      setSavingMode(false)
    }
  }

  const handleChangePassword = async (e: Event) => {
    e.preventDefault()
    setPasswordSuccess('')
    setPasswordError('')
    setNewKeyAfterPasswordChange(null)

    if (newPassword.length < 8) {
      setPasswordError('New password must be at least 8 characters')
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordError('Passwords do not match')
      return
    }

    setSavingPassword(true)
    try {
      const res = await apiFetch<{ new_recovery_key: string }>('/auth/change-password', {
        method: 'POST',
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      })
      setPasswordSuccess('Password updated successfully!')
      setNewKeyAfterPasswordChange(res.new_recovery_key)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      fetchSettings()
    } catch (err: unknown) {
      setPasswordError(err instanceof Error ? err.message : 'Failed to change password')
    } finally {
      setSavingPassword(false)
    }
  }

  const handleRegenerateKey = async () => {
    if (!confirm('Are you sure you want to generate a new emergency recovery key? The old key will be immediately revoked.')) {
      return
    }
    setKeyError('')
    setNewRegeneratedKey(null)
    setRegeneratingKey(true)
    try {
      const res = await apiFetch<{ new_recovery_key: string }>('/auth/recovery-key/regenerate', {
        method: 'POST',
      })
      setNewRegeneratedKey(res.new_recovery_key)
    } catch (err: unknown) {
      setKeyError(err instanceof Error ? err.message : 'Failed to regenerate recovery key')
    } finally {
      setRegeneratingKey(false)
    }
  }

  return (
    <div className={s.page}>
      <div className={s.header}>
        <button type="button" onClick={() => history.back()} className={s.backBtn} aria-label="Back">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={colors.ink} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
        <h1 className={s.title}>Authentication & Security</h1>
      </div>

      {loading && <div>Loading...</div>}

      {settings && (
        <>
          {/* SECTION 1: AUTH MODE */}
          <div className={s.section}>
            <h2 className={s.sectionTitle}>Access Mode</h2>
            <div className={s.sectionDesc}>
              Choose how you sign in to Trackarr. In "Google Only" mode, local password prompts are hidden.
            </div>

            {modeSuccess && <div className={s.alertSuccess}>{modeSuccess}</div>}
            {modeError && <div className={s.alertError}>{modeError}</div>}

            <form onSubmit={handleSaveMode} className={s.form}>
              <div className={s.inputGroup}>
                <label htmlFor="auth-mode" className={s.label}>Active Mode</label>
                <select
                  id="auth-mode"
                  value={authMode}
                  onChange={(e) => setAuthMode((e.target as HTMLSelectElement).value as any)}
                  className={s.input}
                >
                  <option value="hybrid">Hybrid (Local Credentials + Google OAuth)</option>
                  <option value="password" disabled={!settings.has_password}>
                    Local Credentials Only {!settings.has_password && '(password required)'}
                  </option>
                  <option value="google" disabled={!settings.has_google}>
                    Google OAuth Only {!settings.has_google && '(Google not configured)'}
                  </option>
                </select>
              </div>

              <div className={s.inputGroup}>
                <label htmlFor="admin-user" className={s.label}>Local Username</label>
                <input
                  id="admin-user"
                  type="text"
                  value={username}
                  onInput={(e) => setUsername((e.target as HTMLInputElement).value)}
                  className={s.input}
                  required
                />
              </div>

              <button type="submit" disabled={savingMode} className={s.btnPrimary}>
                {savingMode ? 'Saving...' : 'Save Access Mode'}
              </button>
            </form>
          </div>

          {/* SECTION 2: PASSWORD CHANGE */}
          <div className={s.section}>
            <h2 className={s.sectionTitle}>
              {settings.has_password ? 'Change Password' : 'Set Local Password'}
            </h2>
            <div className={s.sectionDesc}>
              Updating your password will automatically regenerate your single-use emergency recovery key.
            </div>

            {passwordSuccess && <div className={s.alertSuccess}>{passwordSuccess}</div>}
            {passwordError && <div className={s.alertError}>{passwordError}</div>}

            {newKeyAfterPasswordChange && (
              <div className={s.keyAlert}>
                <div className={s.keyTitle}>⚠️ New Emergency Recovery Key</div>
                <div className={s.sectionDesc}>
                  Your old recovery key was revoked. Save your new key immediately:
                </div>
                <div className={s.keyBox}>{newKeyAfterPasswordChange}</div>
                <button
                  type="button"
                  onClick={() => navigator.clipboard.writeText(newKeyAfterPasswordChange)}
                  className={s.btnSecondary}
                >
                  Copy Key
                </button>
              </div>
            )}

            <form onSubmit={handleChangePassword} className={s.form}>
              {settings.has_password && (
                <div className={s.inputGroup}>
                  <label htmlFor="curr-pass" className={s.label}>Current Password</label>
                  <input
                    id="curr-pass"
                    type="password"
                    autocomplete="current-password"
                    value={currentPassword}
                    onInput={(e) => setCurrentPassword((e.target as HTMLInputElement).value)}
                    className={s.input}
                    required
                  />
                </div>
              )}

              <div className={s.inputGroup}>
                <label htmlFor="new-pass" className={s.label}>New Password (min. 8 characters)</label>
                <input
                  id="new-pass"
                  type="password"
                  autocomplete="new-password"
                  value={newPassword}
                  onInput={(e) => setNewPassword((e.target as HTMLInputElement).value)}
                  className={s.input}
                  required
                />
              </div>

              <div className={s.inputGroup}>
                <label htmlFor="confirm-pass" className={s.label}>Confirm New Password</label>
                <input
                  id="confirm-pass"
                  type="password"
                  autocomplete="new-password"
                  value={confirmPassword}
                  onInput={(e) => setConfirmPassword((e.target as HTMLInputElement).value)}
                  className={s.input}
                  required
                />
              </div>

              <button type="submit" disabled={savingPassword} className={s.btnPrimary}>
                {savingPassword ? 'Updating...' : 'Update Password'}
              </button>
            </form>
          </div>

          {/* SECTION 3: EMERGENCY RECOVERY KEY */}
          <div className={s.section}>
            <h2 className={s.sectionTitle}>Emergency Recovery Key</h2>
            <div className={s.sectionDesc}>
              If you ever lose access to your account, this key allows you to reset your password without an email server.
            </div>

            {keyError && <div className={s.alertError}>{keyError}</div>}

            {newRegeneratedKey && (
              <div className={s.keyAlert}>
                <div className={s.keyTitle}>🔑 New Recovery Key Generated</div>
                <div className={s.keyBox}>{newRegeneratedKey}</div>
                <button
                  type="button"
                  onClick={() => navigator.clipboard.writeText(newRegeneratedKey)}
                  className={s.btnSecondary}
                >
                  Copy Key
                </button>
              </div>
            )}

            <button
              type="button"
              onClick={handleRegenerateKey}
              disabled={regeneratingKey}
              className={s.btnSecondary}
            >
              {regeneratingKey ? 'Generating...' : 'Generate New Recovery Key'}
            </button>
          </div>
        </>
      )}
    </div>
  )
}
