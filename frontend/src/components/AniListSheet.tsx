import type { Title } from '../types'
import { colors } from '../theme'
import { BottomSheet } from './BottomSheet'

interface AniListSheetProps {
  open: boolean
  onClose: () => void
  title: Title
  onConfirm?: () => void
  onFix?: () => void
}

export function AniListSheet({ open, onClose, title, onConfirm, onFix }: AniListSheetProps) {
  const name = (title.names ?? []).find((n) => n.is_primary)?.name ?? 'Untitled'
  const hasAnilistMatch = !!title.anilist_id

  return (
    <BottomSheet open={open} onClose={onClose}>
      <div style={{ padding: '8px 16px 20px' }}>
        <div style={{ fontSize: '15px', fontWeight: 600, color: colors.textPrimary, marginBottom: '12px' }}>
          AniList Match
        </div>

        {hasAnilistMatch ? (
          <>
            {/* Match card */}
            <div style={{
              background: colors.bgSurface,
              borderRadius: '10px',
              padding: '12px',
              display: 'flex',
              gap: '12px',
              marginBottom: '12px',
            }}>
              <div style={{
                width: '48px',
                height: '68px',
                borderRadius: '6px',
                background: title.cover_url
                  ? `url(/api/covers/${title.cover_url}) center/cover`
                  : `linear-gradient(135deg, ${colors.bgCard}, ${colors.bgSurface})`,
                flexShrink: 0,
              }} />
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: colors.textPrimary }}>{name}</div>
                <div style={{ fontSize: '10px', color: colors.textSecondary, marginTop: '2px' }}>
                  AniList ID: {title.anilist_id}
                </div>
                <a
                  href={`https://anilist.co/anime/${title.anilist_id}`}
                  target="_blank"
                  rel="noopener"
                  style={{ fontSize: '10px', color: colors.accentAnilist, marginTop: '4px', display: 'block' }}
                >
                  View on AniList
                </a>
              </div>
            </div>

            {/* Confidence */}
            <div style={{
              background: title.match_status === 'confirmed' ? `${colors.accentGreen}1F` : `${colors.accentAmber}1F`,
              borderRadius: '8px',
              padding: '8px 12px',
              marginBottom: '16px',
              fontSize: '11px',
              color: title.match_status === 'confirmed' ? colors.accentGreen : colors.accentAmber,
              fontWeight: 500,
            }}>
              {title.match_status === 'confirmed' ? 'Match confirmed' : 'Pending confirmation'}
            </div>

            {/* Actions */}
            <div style={{ display: 'flex', gap: '8px' }}>
              {title.match_status !== 'confirmed' && (
                <button
                  onClick={onConfirm}
                  style={{
                    flex: 1,
                    background: colors.accentAnilist,
                    borderRadius: '12px',
                    padding: '13px',
                    border: 'none',
                    cursor: 'pointer',
                    textAlign: 'center',
                  }}
                >
                  <span style={{ fontSize: '13px', fontWeight: 700, color: '#fff' }}>Confirm & Sync</span>
                </button>
              )}
              <button
                onClick={onFix}
                style={{
                  flex: 1,
                  background: colors.bgSurface,
                  borderRadius: '12px',
                  padding: '13px',
                  border: 'none',
                  cursor: 'pointer',
                  textAlign: 'center',
                }}
              >
                <span style={{ fontSize: '13px', fontWeight: 500, color: colors.textPrimary }}>Wrong match</span>
              </button>
            </div>
          </>
        ) : (
          <div style={{ textAlign: 'center', padding: '20px 0', color: colors.textSecondary, fontSize: '13px' }}>
            No AniList match found. Use the Add screen to search manually.
          </div>
        )}
      </div>
    </BottomSheet>
  )
}
