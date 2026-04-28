import { useEffect, useState } from 'preact/hooks'
import clsx from 'clsx'
import { BottomSheet } from './BottomSheet'
import s from './RatingPrompt.module.css'

interface RatingPromptProps {
  open: boolean
  onClose: () => void
  titleName: string
  context?: string
  initialRating?: number | null
  hasImdb: boolean
  onSave: (rating: number) => void
  onSaveAndImdb?: (rating: number) => void
}

export function RatingPrompt({
  open, onClose, titleName, context, initialRating,
  hasImdb, onSave, onSaveAndImdb,
}: RatingPromptProps) {
  const [rating, setRating] = useState(initialRating ?? 0)

  useEffect(() => {
    if (open) setRating(initialRating ?? 0)
  }, [initialRating, open])

  return (
    <BottomSheet open={open} onClose={onClose}>
      {/* Context */}
      {context && (
        <div className={s.context}>
          <div className={s.contextText}>{context}</div>
        </div>
      )}

      {/* Title */}
      <div className={s.titleWrapper}>
        <span className={s.titleName}>{titleName}</span>
      </div>

      {/* Big rating */}
      <div className={s.bigRating}>
        {rating > 0 ? rating : '–'}
        <span className={s.bigRatingSuffix}>/10</span>
      </div>

      {/* 10 stars */}
      <div className={s.stars}>
        {Array.from({ length: 10 }, (_, i) => (
          <span
            key={i}
            onClick={() => setRating(i + 1)}
            className={clsx(s.star, i < rating && s.starActive)}
          >
            ★
          </span>
        ))}
      </div>

      {/* Buttons */}
      <div className={s.buttons}>
        <button
          onClick={() => { if (rating > 0) onSave(rating) }}
          className={clsx(s.saveButton, rating > 0 && s.saveButtonActive)}
        >
          <span className={clsx(s.saveButtonLabel, rating > 0 && s.saveButtonLabelActive)}>
            Save rating
          </span>
        </button>

        <div className={s.externalButtons}>
          {hasImdb && (
            <button
              onClick={() => { if (rating > 0) onSaveAndImdb?.(rating) }}
              className={s.externalButton}
            >
              <span className={s.imdbLabel}>IMDb</span>
              <span className={s.externalSubLabel}>Save & rate</span>
            </button>
          )}
        </div>
      </div>

      {/* Skip */}
      <div onClick={onClose} className={s.skip}>
        <span className={s.skipLabel}>Skip for now</span>
      </div>
    </BottomSheet>
  )
}
