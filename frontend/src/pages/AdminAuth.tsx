import type { JSX } from 'preact'
import { useEffect, useState } from 'preact/hooks'
import { apiFetch } from '../api'
import { AdminHeader } from '../components/AdminHeader'
import { useTranslation } from '../i18n'
import s from './AdminAuth.module.css'

interface AuthSettingsResponse {
  auth_mode: 'google' | 'password' | 'hybrid'
  has_password: boolean
  has_google: boolean
  google_email?: string
  username: string
}

export function AdminAuth({ path }: { path?: string }): JSX.Element {
  const { t } = useTranslation()
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
      setModeSuccess(t('adminAuth.modeSuccess'))
      fetchSettings()
    } catch (err: unknown) {
      setModeError(err instanceof Error ? err.message : t('adminAuth.modeError'))
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
      setPasswordError(t('adminAuth.passwordMinLength'))
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordError(t('adminAuth.passwordMismatch'))
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
      setPasswordSuccess(t('adminAuth.passwordSuccess'))
      setNewKeyAfterPasswordChange(res.new_recovery_key)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      fetchSettings()
    } catch (err: unknown) {
      setPasswordError(err instanceof Error ? err.message : t('adminAuth.passwordError'))
    } finally {
      setSavingPassword(false)
    }
  }

  const handleRegenerateKey = async () => {
    if (!confirm(t('adminAuth.regenerateConfirm'))) {
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
      setKeyError(err instanceof Error ? err.message : t('adminAuth.keyError'))
    } finally {
      setRegeneratingKey(false)
    }
  }

  return (
    <div className={s.page}>
      <AdminHeader title={t('adminAuth.title')} />

      {loading && <div>{t('common.loading')}</div>}

      {settings && (
        <>
          {/* SECTION 1: AUTH MODE */}
          <div className={s.section}>
            <h2 className={s.sectionTitle}>{t('adminAuth.accessModeTitle')}</h2>
            <div className={s.sectionDesc}>
              {t('adminAuth.accessModeDesc')}
            </div>

            {modeSuccess && <div className={s.alertSuccess}>{modeSuccess}</div>}
            {modeError && <div className={s.alertError}>{modeError}</div>}

            <form onSubmit={handleSaveMode} className={s.form}>
              <div className={s.inputGroup}>
                <label htmlFor="auth-mode" className={s.label}>{t('adminAuth.activeModeLabel')}</label>
                <select
                  id="auth-mode"
                  value={authMode}
                  onChange={(e) => setAuthMode((e.target as HTMLSelectElement).value as AuthSettingsResponse['auth_mode'])}
                  className={s.input}
                >
                  <option value="hybrid">{t('adminAuth.modeHybrid')}</option>
                  <option value="password" disabled={!settings.has_password}>
                    {t('adminAuth.modePasswordOnly')} {!settings.has_password && t('adminAuth.modePasswordRequired')}
                  </option>
                  <option value="google" disabled={!settings.has_google}>
                    {t('adminAuth.modeGoogleOnly')} {!settings.has_google && t('adminAuth.modeGoogleNotConfigured')}
                  </option>
                </select>
              </div>

              <div className={s.inputGroup}>
                <label htmlFor="admin-user" className={s.label}>{t('adminAuth.usernameLabel')}</label>
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
                {savingMode ? t('adminAuth.savingMode') : t('adminAuth.saveMode')}
              </button>
            </form>
          </div>

          {/* SECTION 2: PASSWORD CHANGE */}
          <div className={s.section}>
            <h2 className={s.sectionTitle}>
              {settings.has_password ? t('adminAuth.changePasswordTitle') : t('adminAuth.setPasswordTitle')}
            </h2>
            <div className={s.sectionDesc}>
              {t('adminAuth.passwordDesc')}
            </div>

            {passwordSuccess && <div className={s.alertSuccess}>{passwordSuccess}</div>}
            {passwordError && <div className={s.alertError}>{passwordError}</div>}

            {newKeyAfterPasswordChange && (
              <div className={s.keyAlert}>
                <div className={s.keyTitle}>{t('adminAuth.newKeyTitle')}</div>
                <div className={s.sectionDesc}>
                  {t('adminAuth.newKeyDesc')}
                </div>
                <div className={s.keyBox}>{newKeyAfterPasswordChange}</div>
                <button
                  type="button"
                  onClick={() => void navigator.clipboard.writeText(newKeyAfterPasswordChange)}
                  className={s.btnSecondary}
                >
                  {t('adminAuth.copyKey')}
                </button>
              </div>
            )}

            <form onSubmit={handleChangePassword} className={s.form}>
              {settings.has_password && (
                <div className={s.inputGroup}>
                  <label htmlFor="curr-pass" className={s.label}>{t('adminAuth.currentPasswordLabel')}</label>
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
                <label htmlFor="new-pass" className={s.label}>{t('adminAuth.newPasswordLabel')}</label>
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
                <label htmlFor="confirm-pass" className={s.label}>{t('adminAuth.confirmPasswordLabel')}</label>
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
                {savingPassword ? t('adminAuth.updatingPassword') : t('adminAuth.updatePassword')}
              </button>
            </form>
          </div>

          {/* SECTION 3: EMERGENCY RECOVERY KEY */}
          <div className={s.section}>
            <h2 className={s.sectionTitle}>{t('adminAuth.recoveryKeyTitle')}</h2>
            <div className={s.sectionDesc}>
              {t('adminAuth.recoveryKeyDesc')}
            </div>

            {keyError && <div className={s.alertError}>{keyError}</div>}

            {newRegeneratedKey && (
              <div className={s.keyAlert}>
                <div className={s.keyTitle}>{t('adminAuth.regeneratedKeyTitle')}</div>
                <div className={s.keyBox}>{newRegeneratedKey}</div>
                <button
                  type="button"
                  onClick={() => void navigator.clipboard.writeText(newRegeneratedKey)}
                  className={s.btnSecondary}
                >
                  {t('adminAuth.copyKey')}
                </button>
              </div>
            )}

            <button
              type="button"
              onClick={handleRegenerateKey}
              disabled={regeneratingKey}
              className={s.btnSecondary}
            >
              {regeneratingKey ? t('adminAuth.generatingKey') : t('adminAuth.generateKey')}
            </button>
          </div>
        </>
      )}
    </div>
  )
}
