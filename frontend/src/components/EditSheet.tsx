import { useState, useEffect } from 'preact/hooks'
import clsx from 'clsx'
import type { Title, TitleType, TitleStatus } from '../types'
import { BottomSheet } from './BottomSheet'
import { useTranslation } from '../i18n'
import s from './EditSheet.module.css'

interface EditSheetProps {
  open: boolean
  onClose: () => void
  title: Title
  onSave: (updates: { type?: TitleType; status?: TitleStatus; is_anime?: boolean; delete_from_sonarr?: boolean }) => void
}

const typeOptions: { value: TitleType; key: 'movie' | 'series' }[] = [
  { value: 'movie', key: 'movie' },
  { value: 'series', key: 'series' },
]

const statusOptions: { value: TitleStatus; key: 'watching' | 'completed' | 'dropped' | 'planToWatch' }[] = [
  { value: 'watching', key: 'watching' },
  { value: 'completed', key: 'completed' },
  { value: 'dropped', key: 'dropped' },
  { value: 'plan_to_watch', key: 'planToWatch' },
]

export function EditSheet({ open, onClose, title, onSave }: EditSheetProps) {
  const { t } = useTranslation()
  const [type, setType] = useState<TitleType>(title.type)
  const [isAnime, setIsAnime] = useState<boolean>(title.is_anime)
  const [status, setStatus] = useState<TitleStatus>(title.status)
  const [deleteFromSonarr, setDeleteFromSonarr] = useState<boolean>(false)

  useEffect(() => {
    setType(title.type)
    setIsAnime(title.is_anime)
    setStatus(title.status)
    setDeleteFromSonarr(false)
  }, [title.id, title.status, title.type, title.is_anime])

  const handleStatusChange = (newStatus: TitleStatus) => {
    setStatus(newStatus)
    if (newStatus === 'dropped') {
      if (title.status !== 'dropped') {
        setDeleteFromSonarr(true)
      }
    } else {
      setDeleteFromSonarr(false)
    }
  }

  const handleSave = () => {
    const updates: { type?: TitleType; status?: TitleStatus; is_anime?: boolean; delete_from_sonarr?: boolean } = {}
    if (type !== title.type) updates.type = type
    if (isAnime !== title.is_anime) updates.is_anime = isAnime
    if (status !== title.status) updates.status = status
    if (type === 'series' && status === 'dropped' && deleteFromSonarr) {
      updates.delete_from_sonarr = true
    }
    onSave(updates)
  }

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel={t('editSheet.title')}>
      <div className={s.content}>
        {/* Type selector */}
        <div className={s.section}>
          <div className={s.sectionLabel}>{t('editSheet.type')}</div>
          <div className={s.typeOptions}>
            <button
              type="button"
              onClick={() => setIsAnime(!isAnime)}
              className={clsx(s.typeOption, isAnime && s.activeAnime)}
            >
              {t('editSheet.anime')}
            </button>
            <div className={s.divider} />
            {typeOptions.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => setType(opt.value)}
                className={clsx(s.typeOption, type === opt.value && s.active)}
              >
                {t(`editSheet.${opt.key}`)}
              </button>
            ))}
          </div>
        </div>

        {/* Status selector */}
        <div className={s.section}>
          <div className={s.sectionLabel}>{t('editSheet.status')}</div>
          <div className={s.statusOptions}>
            {statusOptions.map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => handleStatusChange(opt.value)}
                className={clsx(s.statusOption, status === opt.value && s.active)}
              >
                {t(`status.${opt.key}`)}
              </button>
            ))}
          </div>

          {/* Sonarr exclusion / deletion checkbox */}
          {type === 'series' && status === 'dropped' && (
            <div className={s.checkboxContainer}>
              <label className={s.checkboxLabel}>
                <input
                  type="checkbox"
                  checked={deleteFromSonarr}
                  onChange={(e) => setDeleteFromSonarr((e.target as HTMLInputElement).checked)}
                  className={s.checkbox}
                />
                <span className={s.checkboxText}>{t('editSheet.deleteFromSonarr')}</span>
              </label>
            </div>
          )}
        </div>

        {/* Actions */}
        <div className={s.actions}>
          <button type="button" onClick={onClose} className={s.cancelButton}>
            <span className={s.cancelButtonLabel}>{t('common.cancel')}</span>
          </button>
          <button type="button" onClick={handleSave} className={s.saveButton}>
            <span className={s.saveButtonLabel}>{t('common.save')}</span>
          </button>
        </div>
      </div>
    </BottomSheet>
  )
}
