import { useState, useEffect, useRef } from 'preact/hooks'
import { apiFetch } from '../api'
import { useTranslation } from '../i18n'
import s from './PersonalNotesCard.module.css'

interface PersonalNotesCardProps {
  titleId: number
  initialNotes?: string | null
  onSaved?: (notes: string | null) => void
}

type SaveStatus = 'idle' | 'saving' | 'saved' | 'error'

export function PersonalNotesCard({ titleId, initialNotes, onSaved }: PersonalNotesCardProps) {
  const { t } = useTranslation()
  const [text, setText] = useState<string>(initialNotes ?? '')
  const [status, setStatus] = useState<SaveStatus>('idle')
  const debounceTimerRef = useRef<number | null>(null)
  const isFirstMount = useRef(true)

  // Sync state if initialNotes changes from outside
  useEffect(() => {
    if (isFirstMount.current) {
      isFirstMount.current = false
      return
    }
    setText(initialNotes ?? '')
  }, [initialNotes])

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (debounceTimerRef.current !== null) {
        window.clearTimeout(debounceTimerRef.current)
      }
    }
  }, [])

  const saveNotes = async (contentToSave: string) => {
    setStatus('saving')
    try {
      const trimmed = contentToSave.trim()
      const payloadValue = trimmed === '' ? '' : trimmed
      await apiFetch(`/titles/${titleId}`, {
        method: 'PATCH',
        body: JSON.stringify({ personal_notes: payloadValue }),
      })
      setStatus('saved')
      onSaved?.(trimmed === '' ? null : trimmed)
      window.setTimeout(() => {
        setStatus((cur) => (cur === 'saved' ? 'idle' : cur))
      }, 2000)
    } catch {
      setStatus('error')
    }
  }

  const handleChange = (e: Event) => {
    const val = (e.target as HTMLTextAreaElement).value
    setText(val)
    setStatus('saving')

    if (debounceTimerRef.current !== null) {
      window.clearTimeout(debounceTimerRef.current)
    }

    debounceTimerRef.current = window.setTimeout(() => {
      saveNotes(val)
    }, 500)
  }

  const handleBlur = () => {
    if (debounceTimerRef.current !== null) {
      window.clearTimeout(debounceTimerRef.current)
      debounceTimerRef.current = null
      saveNotes(text)
    }
  }

  return (
    <div className={s.card}>
      <div className={s.header}>
        <div className={s.label}>
          <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
            <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
          </svg>
          {t('notes.title')}
        </div>
        {status !== 'idle' && (
          <span
            className={`${s.status} ${
              status === 'saving'
                ? s.statusSaving
                : status === 'saved'
                ? s.statusSaved
                : s.statusError
            }`}
          >
            {status === 'saving' && t('notes.saving')}
            {status === 'saved' && t('notes.saved')}
            {status === 'error' && t('notes.error')}
          </span>
        )}
      </div>
      <textarea
        id="personal_notes"
        name="personal_notes"
        aria-label={t('notes.title')}
        className={s.textarea}
        value={text}
        placeholder={t('notes.placeholder')}
        onInput={handleChange}
        onBlur={handleBlur}
      />
    </div>
  )
}
