import { useState } from 'preact/hooks'
import { BottomSheet } from './BottomSheet'
import { useTranslation } from '../i18n'
import s from './ArrUnlinkDrawer.module.css'

interface ArrUnlinkDrawerProps {
  open: boolean
  appName: string
  onClose: () => void
  onConfirm: (deleteFromArr: boolean) => void | Promise<void>
}

export function ArrUnlinkDrawer({
  open,
  appName,
  onClose,
  onConfirm,
}: ArrUnlinkDrawerProps) {
  const { t } = useTranslation()
  const [pending, setPending] = useState(false)

  const handleAction = async (deleteFromArr: boolean) => {
    if (pending) return
    setPending(true)
    try {
      await onConfirm(deleteFromArr)
      onClose()
    } catch (err) {
      console.error('Arr unlink action failed:', err)
    } finally {
      setPending(false)
    }
  }

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel={t('rematch.arrUnlinkTitle', { app: appName })}>
      <div className={s.content}>
        <div className={s.title}>{t('rematch.arrUnlinkTitle', { app: appName })}</div>
        <div className={s.description}>
          {t('rematch.arrUnlinkDesc', { app: appName })}
        </div>
        <div className={s.actions}>
          <button
            type="button"
            className={s.deleteBtn}
            onClick={() => handleAction(true)}
            disabled={pending}
          >
            {pending ? t('common.loading') : t('rematch.arrDeleteAndRematch', { app: appName })}
          </button>
          <button
            type="button"
            className={s.unlinkBtn}
            onClick={() => handleAction(false)}
            disabled={pending}
          >
            {pending ? t('common.loading') : t('rematch.arrUnlinkOnly')}
          </button>
          <button
            type="button"
            className={s.cancelBtn}
            onClick={onClose}
            disabled={pending}
          >
            {t('common.cancel')}
          </button>
        </div>
      </div>
    </BottomSheet>
  )
}
