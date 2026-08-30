import { useState, useEffect } from 'preact/hooks'
import { apiFetch } from '../api'
import type { CalendarTokenResponse } from '../types'
import { ErrorBanner } from './ErrorBanner'
import s from './CalendarIcalModal.module.css'

interface Props {
  isOpen: boolean
  onClose: () => void
}

export function CalendarIcalModal({ isOpen, onClose }: Props) {
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
        setError(err instanceof Error ? err.message : 'Impossible de charger le flux iCal')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [isOpen])

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
      setError('Impossible de copier automatiquement dans le presse-papier')
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
      setError(err instanceof Error ? err.message : 'Échec de la régénération du token')
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
            <span>Abonnement au Flux iCal</span>
          </div>
          <button onClick={onClose} aria-label="Fermer" className={s.closeBtn}>
            ✕
          </button>
        </div>

        {/* Content */}
        <div className={s.body}>
          {error && <ErrorBanner message={error} onDismiss={() => setError(null)} />}

          <p className={s.desc}>
            Abonnez-vous à votre flux Trackarr pour afficher automatiquement les épisodes et films à venir dans votre calendrier (Apple Calendar, Google Calendar, Outlook).
          </p>

          {loading && !tokenData ? (
            <div className={s.loadingPlaceholder}>Génération du flux en cours...</div>
          ) : tokenData ? (
            <>
              {/* URL Box */}
              <div className={s.urlSection}>
                <label className={s.urlLabel}>Lien d'abonnement personnel</label>
                <div className={s.urlBox}>
                  <input
                    type="text"
                    readOnly
                    value={tokenData.http_url}
                    className={s.urlInput}
                    onClick={(e) => (e.target as HTMLInputElement).select()}
                  />
                  <button onClick={handleCopy} className={s.copyBtn}>
                    {copied ? '✓ Copié !' : 'Copier'}
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
                <div className={s.guideTitle}>Comment s'abonner :</div>
                <ul className={s.guideList}>
                  <li>
                    <strong>Apple Calendar (iOS / macOS) :</strong> Cliquez sur le bouton <em>Apple Calendar</em> ou allez dans <em>Fichier → Nouvel abonnement à un calendrier</em> et collez l'URL.
                  </li>
                  <li>
                    <strong>Google Calendar :</strong> Cliquez sur <em>Google Calendar</em> ou dans les paramètres de Google Agenda, cliquez sur <em>Autres agendas (+) → À partir de l'URL</em>.
                  </li>
                  <li>
                    <strong>Outlook :</strong> Allez dans <em>Ajouter un calendrier → S'abonner à partir du web</em> et collez l'URL.
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
              {confirmRegen ? '⚠️ Confirmer la réinitialisation du token' : 'Regénérer le lien secret'}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
