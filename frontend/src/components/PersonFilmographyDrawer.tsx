import { route } from 'preact-router'
import { BottomSheet } from './BottomSheet'
import { TitleCard } from './TitleCard'
import { ErrorBanner } from './ErrorBanner'
import { useApi } from '../hooks/useApi'
import { routeTo } from '../routes'
import { useTranslation } from '../i18n'
import type { PaginatedResponse } from '../types'
import s from './PersonFilmographyDrawer.module.css'

interface Props {
  open: boolean
  personName: string | null
  role?: 'actor' | 'director'
  onClose: () => void
}

export function PersonFilmographyDrawer({ open, personName, role, onClose }: Props) {
  const { t } = useTranslation()
  const { data, error, loading, mutate } = useApi<PaginatedResponse>(
    open && personName ? `/titles?person=${encodeURIComponent(personName)}&limit=100` : null
  )

  const titles = data?.titles ?? []
  const roleIcon = role === 'director' ? '🎥' : '🎬'

  const handleViewAll = (e: MouseEvent) => {
    e.preventDefault()
    if (personName) {
      onClose()
      route(routeTo.person(personName))
    }
  }

  return (
    <BottomSheet open={open} onClose={onClose} ariaLabel={personName ?? t('stats.filmography')}>
      <div className={s.content}>
        {/* Header */}
        <div className={s.header}>
          <div className={s.headerLeft}>
            <div className={s.nameRow}>
              <span className={s.roleIcon} aria-hidden="true">{roleIcon}</span>
              <span className={s.name}>{personName}</span>
            </div>
            <div className={s.subtitle}>
              {titles.length > 0
                ? t('stats.filmographyCount', {
                    count: titles.length,
                    plural: titles.length === 1 ? '' : 's',
                  })
                : role === 'director'
                  ? t('stats.director')
                  : t('stats.actor')}
            </div>
          </div>

          {personName && (
            <a
              href={routeTo.person(personName)}
              onClick={handleViewAll}
              className={s.viewAllBtn}
            >
              <span>{t('stats.viewPersonPage')}</span>
              <span aria-hidden="true">→</span>
            </a>
          )}
        </div>

        {/* Error banner */}
        {error && <ErrorBanner message={error} onRetry={mutate} />}

        {/* Body list */}
        <div className={s.listWrapper}>
          {loading && <div className={s.loading}>{t('common.loading')}</div>}

          {!loading && !error && titles.length === 0 && (
            <div className={s.empty}>{t('stats.noTitles')}</div>
          )}

          {!loading && titles.length > 0 && (
            titles.map((title) => (
              <TitleCard
                key={title.id}
                title={title}
                onUpdate={mutate}
                showSortCaption={false}
              />
            ))
          )}
        </div>
      </div>
    </BottomSheet>
  )
}
