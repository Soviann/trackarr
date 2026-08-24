import clsx from 'clsx'
import type { Title } from '../types'
import { aniListMediaUrl, getName, getCoverUrl } from '../utils'
import { BottomSheet } from './BottomSheet'
import s from './AniListSheet.module.css'

interface AniListSheetProps {
  open: boolean
  onClose: () => void
  title: Title
  onConfirm?: () => void
  onFix?: () => void
}

export function AniListSheet({ open, onClose, title, onConfirm, onFix }: AniListSheetProps) {
  const name = getName(title)
  const hasAnilistMatch = !!title.anilist_id
  const isConfirmed = title.match_status === 'confirmed'
  const coverUrl = getCoverUrl(title.cover_url)

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel="AniList match">
      <div className={s.container}>
        <div className={s.heading}>
          AniList Match
        </div>

        {hasAnilistMatch ? (
          <>
            {/* Match card */}
            <div className={s.matchCard}>
              <div
                className={clsx(s.cover, !coverUrl && s.coverFallback)}
                style={coverUrl
                  ? { background: `url(${coverUrl}) center/cover` }
                  : undefined}
              />
              <div>
                <div className={s.titleName}>{name}</div>
                <div className={s.titleId}>
                  AniList ID: {title.anilist_id}
                </div>
                <a
                  href={aniListMediaUrl(title.anilist_id!)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={s.anilistLink}
                >
                  View on AniList
                </a>
              </div>
            </div>

            {/* Confidence */}
            <div className={clsx(s.confidence, isConfirmed ? s.confirmed : s.pending)}>
              {isConfirmed ? 'Match confirmed' : 'Pending confirmation'}
            </div>

            {/* Actions */}
            <div className={s.actions}>
              {!isConfirmed && (
                <button onClick={onConfirm} className={s.btnConfirm}>
                  <span className={s.btnConfirmLabel}>Confirm & Sync</span>
                </button>
              )}
              <button onClick={onFix} className={s.btnWrong}>
                <span className={s.btnWrongLabel}>Wrong match</span>
              </button>
            </div>
          </>
        ) : (
          <div className={s.empty}>
            No AniList match found. Use the Add screen to search manually.
          </div>
        )}
      </div>
    </BottomSheet>
  )
}
