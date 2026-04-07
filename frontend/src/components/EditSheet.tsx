import { useState } from 'preact/hooks'
import clsx from 'clsx'
import type { Title, TitleType, TitleStatus } from '../types'
import { BottomSheet } from './BottomSheet'
import s from './EditSheet.module.css'

interface EditSheetProps {
  open: boolean
  onClose: () => void
  title: Title
  onSave: (updates: { type?: TitleType; status?: TitleStatus; is_anime?: boolean }) => void
}

const typeOptions: { value: TitleType; label: string }[] = [
  { value: 'movie', label: 'Movie' },
  { value: 'series', label: 'Series' },
]

const statusOptions: { value: TitleStatus; label: string }[] = [
  { value: 'watching', label: 'Watching' },
  { value: 'completed', label: 'Completed' },
  { value: 'dropped', label: 'Dropped' },
  { value: 'plan_to_watch', label: 'Plan to watch' },
]

export function EditSheet({ open, onClose, title, onSave }: EditSheetProps) {
  const [type, setType] = useState<TitleType>(title.type)
  const [isAnime, setIsAnime] = useState<boolean>(title.is_anime)
  const [status, setStatus] = useState<TitleStatus>(title.status)

  const handleSave = () => {
    const updates: { type?: TitleType; status?: TitleStatus; is_anime?: boolean } = {}
    if (type !== title.type) updates.type = type
    if (isAnime !== title.is_anime) updates.is_anime = isAnime
    if (status !== title.status) updates.status = status
    onSave(updates)
  }

  return (
    <BottomSheet open={open} onClose={onClose}>
      <div className={s.content}>
        {/* Type selector */}
        <div className={s.section}>
          <div className={s.sectionLabel}>Type</div>
          <div className={s.typeOptions}>
            <button
              onClick={() => setIsAnime(!isAnime)}
              className={clsx(s.typeOption, isAnime && s.activeAnime)}
            >
              Anime
            </button>
            <div className={s.divider} />
            {typeOptions.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setType(opt.value)}
                className={clsx(s.typeOption, type === opt.value && s.active)}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>

        {/* Status selector */}
        <div className={s.section}>
          <div className={s.sectionLabel}>Status</div>
          <div className={s.statusOptions}>
            {statusOptions.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setStatus(opt.value)}
                className={clsx(s.statusOption, status === opt.value && s.active)}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>

        {/* Actions */}
        <div className={s.actions}>
          <button onClick={onClose} className={s.cancelButton}>
            <span className={s.cancelButtonLabel}>Cancel</span>
          </button>
          <button onClick={handleSave} className={s.saveButton}>
            <span className={s.saveButtonLabel}>Save</span>
          </button>
        </div>
      </div>
    </BottomSheet>
  )
}
