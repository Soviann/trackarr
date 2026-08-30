import { useState, useEffect } from 'preact/hooks'
import { apiFetch } from '../api'
import type { CalendarTokenResponse } from '../types'
import { useTranslation } from '../i18n'
import { ErrorBanner } from './ErrorBanner'
import s from './CalendarIcalModal.module.css'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export function CalendarIcalModal({ isOpen, onClose }: Props) {
  const { t } = useTranslation()
  const [tokenData, setTokenData] = useState<CalendarTokenResponse | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [confirmRegen, setConfirmRegen] = useState(false)

  useEffect(() => {
    if (!isOpen) return

    setLoading(true)
    setError(null)
    apiFetch<CalendarTokenResponse>('/calendar/token')
      .then((data) => {
        setTokenData(data)
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : t('calendar.loadError'))
      })
      .finally(() => {
        setLoading(false)
      })
  }, [isOpen, t])

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isOpen, onClose])

  if (!isOpen) return null

  const handleCopy = async () => {
    if (!tokenData?.http_url) return
    try {
      await navigator.clipboard.writeText(tokenData.http_url)
      setCopied(true)
      setTimeout(() => setCopied(false), 2500)
    } catch {
      setError(t('calendar.loadError'))
    }
  }

  const handleRegenerate = async () => {
    if (!confirmRegen) {
      setConfirmRegen(true)
      return
    }

    setLoading(true)
    setError(null)
    setConfirmRegen(false)

    try {
      const data = await apiFetch<CalendarTokenResponse>('/calendar/token/regenerate', {
        method: 'POST',
      })
      setTokenData(data)
    } catch (err) {
      setError(err instanceof Error ? err.message : t('calendar.loadError'))
    } finally {
      setLoading(false)
    }
  }

  const googleCalUrl = tokenData?.webcal_url
    ? `https://calendar.google.com/calendar/r?cid=${encodeURIComponent(tokenData.webcal_url)}`
    : '#'

  return (
    <div className={s.overlay} onClick={onClose}>
      <div className={s.modal} onClick={(e) => e.stopPropagation()}>
        {/* Header */}
        <div className={s.header}>
          <div className={s.headerTitle}>
            <span>📅</span>
            <span>{t('calendar.icalModalTitle')}</span>
          </div>
          <button onClick={onClose} aria-label={t('common.close')} className={s.closeBtn}>
            ✕
          </button>
        </div>

        {/* Content */}
        <div className={s.body}>
          {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

          <p className={s.desc}>
            {t('calendar.icalModalDesc')}
          </p>

          {loading && !tokenData ? (
            <div className={s.loadingPlaceholder}>{t('common.loading')}</div>
          ) : tokenData ? (
            <>
              {/* URL Box */}
              <div className={s.urlSection}>
                <label className={s.urlLabel}>{t('calendar.personalUrl')}</label>
                <div className={s.urlBox}>
                  <input
                    type="text"
                    readOnly
                    value={tokenData.http_url}
                    className={s.urlInput}
                    onClick={(e) => (e.target as HTMLInputElement).select()}
                  />
                  <button onClick={handleCopy} className={s.copyBtn}>
                    {copied ? t('calendar.copied') : t('calendar.copyUrl')}
                  </button>
                </div>
              </div>

              {/* Quick Subscribe Actions */}
              <div className={s.quickActions}>
                <a
                  href={tokenData.webcal_url}
                  className={s.actionBtn}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <span>🍎</span>
                  <span>Apple Calendar</span>
                </a>
                <a
                  href={googleCalUrl}
                  className={s.actionBtn}
                  target="_blank"
                  rel="noopener noreferrer"
                >
                  <span>🌐</span>
                  <span>Google Calendar</span>
                </a>
              </div>

              {/* Step-by-step Guides */}
              <div className={s.guideSection}>
                <div className={s.guideTitle}>{t('calendar.howToSubscribe')}</div>
                <ul className={s.guideList}>
                  <li>
                    {t('calendar.appleGuide')}
                  </li>
                  <li>
                    {t('calendar.googleGuide')}
                  </li>
                  <li>
                    {t('calendar.outlookGuide')}
                  </li>
                </ul>
              </div>
            </>
          ) : null}
        </div>

        {/* Footer */}
        {tokenData && (
          <div className={s.footer}>
            <button onClick={handleRegenerate} className={s.regenBtn}>
              {confirmRegen ? t('calendar.regenConfirm') : t('calendar.regenToken')}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
