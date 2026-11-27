import { route } from 'preact-router'
import { BottomSheet } from './BottomSheet'
import { TitleCard } from './TitleCard'
import { ErrorBanner } from './ErrorBanner'
import { useApi } from '../hooks/useApi'
import { routeTo } from '../routes'
import { useTranslation, type TranslationKey } from '../i18n'
import type { PaginatedResponse, PersonRole } from '../types'
import s from './PersonFilmographyDrawer.module.css'

export interface PersonFilmographyDrawerProps {
  open: boolean
  personName: string | null
  role?: PersonRole
  onClose: () => void
}

const ROLE_META: Record<PersonRole, { icon: string; labelKey: TranslationKey }> = {
  actor: { icon: '🎬', labelKey: 'stats.actor' },
  director: { icon: '🎥', labelKey: 'stats.director' },
  writer: { icon: '✍️', labelKey: 'stats.writer' },
  composer: { icon: '🎵', labelKey: 'stats.composer' },
}

export function PersonFilmographyDrawer({ open, personName, role, onClose }: PersonFilmographyDrawerProps) {
  const { t } = useTranslation()
  const requestPath = open && personName
    ? `/titles?person=${encodeURIComponent(personName)}${role ? `&role=${encodeURIComponent(role)}` : ''}&limit=100`
    : null
  const { data, error, loading, mutate } = useApi<PaginatedResponse>(requestPath)

  const titles = data?.titles ?? []
  const roleMeta = role ? ROLE_META[role] : undefined
  const roleIcon = roleMeta?.icon ?? '🎬'
  const roleLabel = roleMeta ? t(roleMeta.labelKey) : ''
  const personUrl = personName ? routeTo.person(personName, role) : undefined

  const handleViewAll = (e: MouseEvent) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button === 1) {
      onClose()
      return
    }
    e.preventDefault()
    e.stopPropagation()
    if (personUrl) {
      onClose()
      route(personUrl)
    }
  }

  const countText = t('stats.filmographyCount', {
    count: titles.length,
    plural: titles.length === 1 ? '' : 's',
  })
  const subtitle = titles.length > 0
    ? (roleLabel ? `${roleLabel} · ${countText}` : countText)
    : (roleLabel || t('stats.filmography'))

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
            <div className={s.subtitle}>{subtitle}</div>
          </div>

          {personUrl && (
            <a
              href={personUrl}
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
