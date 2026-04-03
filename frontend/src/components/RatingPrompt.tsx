import { useState } from 'preact/hooks'
import { colors } from '../theme'
import { BottomSheet } from './BottomSheet'

interface RatingPromptProps {
  open: boolean
  onClose: () => void
  titleName: string
  context?: string
  initialRating?: number | null
  hasImdb: boolean
  hasAnilist: boolean
  onSave: (rating: number) => void
  onSaveAndImdb?: (rating: number) => void
  onSaveAndAnilist?: (rating: number) => void
}

export function RatingPrompt({
  open, onClose, titleName, context, initialRating,
  hasImdb, hasAnilist, onSave, onSaveAndImdb, onSaveAndAnilist,
}: RatingPromptProps) {
  const [rating, setRating] = useState(initialRating ?? 0)

  return (
    <BottomSheet open={open} onClose={onClose}>
      {/* Context */}
      {context && (
        <div style={{ textAlign: 'center', padding: '8px 16px 2px' }}>
          <div style={{ fontSize: '15px', fontWeight: 600, color: colors.textPrimary }}>{context}</div>
        </div>
      )}

      {/* Title */}
      <div style={{ textAlign: 'center', padding: '2px 16px 16px' }}>
        <span style={{
          fontSize: '12px',
          color: colors.accentAmber,
          textDecoration: 'underline',
          textDecorationColor: 'rgba(232,169,37,0.3)',
          textUnderlineOffset: '2px',
        }}>
          {titleName}
        </span>
      </div>

      {/* Big rating */}
      <div style={{ textAlign: 'center', fontSize: '32px', fontWeight: 700, color: colors.accentAmber, padding: '0 0 8px' }}>
        {rating > 0 ? rating : '–'}
        <span style={{ fontSize: '16px', color: colors.textMuted, fontWeight: 400 }}>/10</span>
      </div>

      {/* 10 stars */}
      <div style={{ display: 'flex', justifyContent: 'center', gap: '6px', padding: '0 20px 20px' }}>
        {Array.from({ length: 10 }, (_, i) => (
          <span
            key={i}
            onClick={() => setRating(i + 1)}
            style={{
              fontSize: '24px',
              color: i < rating ? colors.accentAmber : '#333',
              cursor: 'pointer',
              WebkitTapHighlightColor: 'transparent',
            }}
          >
            ★
          </span>
        ))}
      </div>

      {/* Buttons */}
      <div style={{ padding: '0 16px 6px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
        <button
          onClick={() => { if (rating > 0) onSave(rating) }}
          style={{
            background: rating > 0 ? colors.accentAmber : colors.bgSurface,
            borderRadius: '12px',
            padding: '13px',
            textAlign: 'center',
            border: 'none',
            cursor: rating > 0 ? 'pointer' : 'default',
          }}
        >
          <span style={{ fontSize: '13px', fontWeight: 700, color: rating > 0 ? colors.bgPrimary : colors.textMuted }}>
            Save rating
          </span>
        </button>

        <div style={{ display: 'flex', gap: '8px' }}>
          {hasImdb && (
            <button
              onClick={() => { if (rating > 0) onSaveAndImdb?.(rating) }}
              style={{
                flex: 1,
                background: colors.bgSurface,
                borderRadius: '12px',
                padding: '12px',
                textAlign: 'center',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: '6px',
                border: 'none',
                cursor: 'pointer',
              }}
            >
              <span style={{ fontSize: '11px', fontWeight: 800, color: colors.accentImdb, fontFamily: 'Impact,system-ui' }}>IMDb</span>
              <span style={{ fontSize: '11px', color: '#888' }}>Save & rate</span>
            </button>
          )}
          {hasAnilist && (
            <button
              onClick={() => { if (rating > 0) onSaveAndAnilist?.(rating) }}
              style={{
                flex: 1,
                background: colors.bgSurface,
                borderRadius: '12px',
                padding: '12px',
                textAlign: 'center',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: '6px',
                border: 'none',
                cursor: 'pointer',
              }}
            >
              <span style={{ fontSize: '11px', fontWeight: 700, color: colors.accentAnilist }}>AL</span>
              <span style={{ fontSize: '11px', color: '#888' }}>Save & sync</span>
            </button>
          )}
        </div>
      </div>

      {/* Skip */}
      <div
        onClick={onClose}
        style={{ textAlign: 'center', padding: '10px 0 20px', cursor: 'pointer' }}
      >
        <span style={{ fontSize: '12px', color: colors.textMuted }}>Skip for now</span>
      </div>
    </BottomSheet>
  )
}
