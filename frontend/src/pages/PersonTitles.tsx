import { useEffect, useState } from 'preact/hooks'
import clsx from 'clsx'
import { useApi } from '../hooks/useApi'
import type { PaginatedResponse, PersonRoleFilter } from '../types'
import { routeTo } from '../routes'
import { TitleCard } from '../components/TitleCard'
import { ErrorBanner } from '../components/ErrorBanner'
import { useTranslation, type TranslationKey } from '../i18n'
import s from './PersonTitles.module.css'

import { useScrollRestoration } from '../hooks/useScrollRestoration'

const ROLE_FILTERS: ReadonlyArray<{ role: PersonRoleFilter; labelKey: TranslationKey }> = [
  { role: 'all', labelKey: 'person.roleAll' },
  { role: 'actor', labelKey: 'stats.actor' },
  { role: 'director', labelKey: 'stats.director' },
  { role: 'writer', labelKey: 'stats.writer' },
  { role: 'composer', labelKey: 'stats.composer' },
]

function parseRoleFilter(raw: string | null | undefined): PersonRoleFilter {
  const normalized = raw?.trim().toLowerCase()
  if (
    normalized === 'actor' ||
    normalized === 'director' ||
    normalized === 'writer' ||
    normalized === 'composer'
  ) {
    return normalized
  }
  return 'all'
}

function readRoleFromLocation(
  url?: string,
  matches?: Record<string, string | undefined>
): PersonRoleFilter {
  if (url !== undefined) {
    const qIdx = url.indexOf('?')
    if (qIdx >= 0) {
      return parseRoleFilter(new URLSearchParams(url.slice(qIdx)).get('role'))
    }
    if (matches?.role) {
      return parseRoleFilter(matches.role)
    }
    return 'all'
  }
  if (matches?.role) {
    return parseRoleFilter(matches.role)
  }
  if (typeof window !== 'undefined' && window.location.search) {
    return parseRoleFilter(new URLSearchParams(window.location.search).get('role'))
  }
  return 'all'
}

export interface PersonTitlesProps {
  path?: string
  name?: string
  url?: string
  matches?: Record<string, string | undefined>
}

export function PersonTitles({ name, url, matches }: PersonTitlesProps) {
  const { t } = useTranslation()
  const [selectedRole, setSelectedRole] = useState<PersonRoleFilter>(() =>
    readRoleFromLocation(url, matches)
  )

  useEffect(() => {
    setSelectedRole(readRoleFromLocation(url, matches))
  }, [name, url, matches?.role])

  const handleSelectRole = (nextRole: PersonRoleFilter) => {
    setSelectedRole(nextRole)
    if (name && typeof window !== 'undefined' && window.history?.replaceState) {
      window.history.replaceState(window.history.state, '', routeTo.person(name, nextRole))
    }
  }

  const requestPath = name
    ? `/titles?person=${encodeURIComponent(name)}${
        selectedRole !== 'all' ? `&role=${encodeURIComponent(selectedRole)}` : ''
      }&limit=200`
    : null

  const { data, error, loading, mutate } = useApi<PaginatedResponse>(requestPath)
  useScrollRestoration(`person-${name ?? ''}`, !loading)

  const titles = data?.titles ?? []

  return (
    <div className={s.page}>
      <div className={s.header}>
        <button type="button" className={s.backBtn} onClick={() => history.back()} aria-label={t('common.back')}>
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <polyline points="15 18 9 12 15 6" />
          </svg>
        </button>
        <div className={s.personName}>{name}</div>
      </div>

      <div className={s.rolePills} role="group" aria-label={t('person.roleFilterAria')}>
        {ROLE_FILTERS.map(({ role, labelKey }) => {
          const active = selectedRole === role
          return (
            <button
              key={role}
              type="button"
              className={clsx(s.rolePill, active && s.rolePillActive)}
              aria-pressed={active}
              onClick={() => handleSelectRole(role)}
            >
              {t(labelKey)}
            </button>
          )
        })}
      </div>

      {error && <ErrorBanner message={error} onRetry={mutate} />}

      {loading && <div className={s.centered}>{t('common.loading')}</div>}

      {!loading && !error && titles.length === 0 && (
        <div className={s.centered}>
          {selectedRole === 'all' ? t('common.noTitlesFound') : t('person.emptyRole')}
        </div>
      )}

      {titles.length > 0 && (
        <div className={s.list}>
          {titles.map((title) => (
            <TitleCard key={title.id} title={title} onUpdate={mutate} showSortCaption={false} />
          ))}
        </div>
      )}
    </div>
  )
}
