import clsx from 'clsx'
import type { Title } from '../types'
import { aniListMediaUrl, getName, getCoverUrl } from '../utils'
import { useTranslation } from '../i18n'
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
  const { t } = useTranslation()
  const name = getName(title)
  const hasAnilistMatch = !!title.anilist_id
  const isConfirmed = title.match_status === 'confirmed'
  const coverUrl = getCoverUrl(title.cover_url)

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel={t('anilistSheet.ariaLabel')}>
      <div className={s.container}>
        <div className={s.heading}>
          {t('anilistSheet.heading')}
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
                  {t('anilistSheet.anilistId', { id: title.anilist_id! })}
                </div>
                <a
                  href={aniListMediaUrl(title.anilist_id!)}
                  target="_blank"
                  rel="noopener noreferrer"
                  className={s.anilistLink}
                >
                  {t('anilistSheet.viewOnAniList')}
                </a>
              </div>
            </div>

            {/* Confidence */}
            <div className={clsx(s.confidence, isConfirmed ? s.confirmed : s.pending)}>
              {isConfirmed ? t('anilistSheet.matchConfirmed') : t('anilistSheet.pendingConfirmation')}
            </div>

            {/* Actions */}
            <div className={s.actions}>
              {!isConfirmed && (
                <button onClick={onConfirm} className={s.btnConfirm}>
                  <span className={s.btnConfirmLabel}>{t('anilistSheet.confirmAndSync')}</span>
                </button>
              )}
              <button onClick={onFix} className={s.btnWrong}>
                <span className={s.btnWrongLabel}>{t('anilistSheet.wrongMatch')}</span>
              </button>
            </div>
          </>
        ) : (
          <div className={s.empty}>
            {t('anilistSheet.noMatch')}
          </div>
        )}
      </div>
    </BottomSheet>
  )
}
