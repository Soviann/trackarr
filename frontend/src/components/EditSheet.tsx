import { useState } from 'preact/hooks'
import type { Title, TitleType, TitleStatus } from '../types'
import { colors } from '../theme'
import { BottomSheet } from './BottomSheet'

interface EditSheetProps {
  open: boolean
  onClose: () => void
  title: Title
  onSave: (updates: { type?: TitleType; status?: TitleStatus }) => void
}

const typeOptions: { value: TitleType; label: string }[] = [
  { value: 'movie', label: 'Movie' },
  { value: 'series', label: 'Series' },
  { value: 'anime', label: 'Anime' },
]

const statusOptions: { value: TitleStatus; label: string }[] = [
  { value: 'watching', label: 'Watching' },
  { value: 'completed', label: 'Completed' },
  { value: 'dropped', label: 'Dropped' },
  { value: 'plan_to_watch', label: 'Plan to watch' },
]

export function EditSheet({ open, onClose, title, onSave }: EditSheetProps) {
  const [type, setType] = useState<TitleType>(title.type)
  const [status, setStatus] = useState<TitleStatus>(title.status)

  const handleSave = () => {
    const updates: { type?: TitleType; status?: TitleStatus } = {}
    if (type !== title.type) updates.type = type
    if (status !== title.status) updates.status = status
    onSave(updates)
  }

  return (
    <BottomSheet open={open} onClose={onClose}>
      <div style={{ padding: '8px 16px 20px' }}>
        {/* Type selector */}
        <div style={{ marginBottom: '16px' }}>
          <div style={{ fontSize: '11px', color: colors.textSecondary, fontWeight: 600, marginBottom: '8px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
            Type
          </div>
          <div style={{ display: 'flex', gap: '8px' }}>
            {typeOptions.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setType(opt.value)}
                style={{
                  flex: 1,
                  padding: '10px',
                  borderRadius: '10px',
                  border: `1px solid ${type === opt.value ? colors.accentAmber : colors.borderCard}`,
                  background: type === opt.value ? `${colors.accentAmber}1F` : colors.bgSurface,
                  color: type === opt.value ? colors.accentAmber : colors.textSecondary,
                  fontSize: '12px',
                  fontWeight: type === opt.value ? 600 : 400,
                  cursor: 'pointer',
                  textAlign: 'center',
                }}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>

        {/* Status selector */}
        <div style={{ marginBottom: '16px' }}>
          <div style={{ fontSize: '11px', color: colors.textSecondary, fontWeight: 600, marginBottom: '8px', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
            Status
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            {statusOptions.map((opt) => (
              <button
                key={opt.value}
                onClick={() => setStatus(opt.value)}
                style={{
                  padding: '12px',
                  borderRadius: '10px',
                  border: `1px solid ${status === opt.value ? colors.accentAmber : colors.borderCard}`,
                  background: status === opt.value ? `${colors.accentAmber}1F` : colors.bgSurface,
                  color: status === opt.value ? colors.accentAmber : colors.textPrimary,
                  fontSize: '13px',
                  fontWeight: status === opt.value ? 600 : 400,
                  cursor: 'pointer',
                  textAlign: 'left',
                }}
              >
                {opt.label}
              </button>
            ))}
          </div>
        </div>

        {/* Save */}
        <button
          onClick={handleSave}
          style={{
            width: '100%',
            background: colors.accentAmber,
            borderRadius: '12px',
            padding: '13px',
            border: 'none',
            cursor: 'pointer',
            textAlign: 'center',
          }}
        >
          <span style={{ fontSize: '13px', fontWeight: 700, color: colors.bgPrimary }}>Save</span>
        </button>
      </div>
    </BottomSheet>
  )
}
