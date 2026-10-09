import { useState, useEffect, useCallback } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title, TitleType, TitleStatus, PaginatedResponse, MatchResult } from '../types'
import { useApi } from '../hooks/useApi'
import { useSearchStore } from '../store'
import { getName } from '../utils'
import { isUrl } from '../utils/url'
import { StatusBadge } from '../components/StatusBadge'
import { apiFetch } from '../api'
import { colors } from '../theme'
import { CoverImage } from '../components/CoverImage'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import clsx from 'clsx'
import { PullToRefresh } from '../components/PullToRefresh'
import { useTranslation } from '../i18n'
import s from './Validate.module.css'

interface RematchPayload {
  tmdb_id?: number
  imdb_id?: string
  anilist_id?: number
  type?: TitleType
}

interface AddTitlePayload {
  status: TitleStatus
  match_status: string
  type?: string
  is_anime?: boolean
  year?: number
  names?: { name: string; language: string; is_primary: boolean }[]
  imdb_id?: string
  tmdb_id?: number
  tvdb_id?: number
  anilist_id?: number
  cover_url?: string | null
}

export function Validate({ path }: { path?: string }) {
  const { t } = useTranslation()
  const params = new URLSearchParams(window.location.search)
  const query = params.get('q') ?? ''
  const id = params.get('id')
  // Share fallback: when /add ships us here from an IMDb/AniList share, it
  // forwards the human title under `name`. If URL resolve later fails we use
  // it to bounce the user to the in-app TMDB search instead of stranding them
  // on a useless "add with empty data" card.
  const fallbackName = params.get('name')

  const searchPath = query && !isUrl(query) ? `/titles?search=${encodeURIComponent(query)}` : null
  const { data: resultsData, loading: loadingSearch, mutate: mutateSearch } = useApi<PaginatedResponse>(searchPath)
  const results = resultsData?.titles ?? []

  const { data: currentTitle, loading: loadingCurrent, mutate: mutateCurrent } = useApi<Title>(id ? `/titles/${id}` : null)
  
  const [inputValue, setInputValue] = useState(query)
  const [adding, setAdding] = useState(false)
  const [selectedStatus, setSelectedStatus] = useState<TitleStatus>('plan_to_watch')
  const [resolved, setResolved] = useState<MatchResult | null>(null)
  const [loadingResolve, setLoadingResolve] = useState(false)

  // Merge state
  const [mergeTarget, setMergeTarget] = useState<Title | null>(null)

  useEffect(() => {
    setSelectedStatus(currentTitle?.status ?? 'plan_to_watch')
  }, [currentTitle])

  useEffect(() => {
    if (isUrl(query)) {
      setLoadingResolve(true)
      apiFetch<MatchResult>(`/titles/resolve?q=${encodeURIComponent(query)}`)
        .then(setResolved)
        .catch(() => {
          setResolved(null)
          // Adding a title TMDB doesn't know is useless (no metadata edit in
          // Trackarr). Redirect to the in-app TMDB search prefilled with
          // the share-provided name so the user can pick the canonical entry.
          if (fallbackName) {
            useSearchStore.getState().setQuery(fallbackName)
            useSearchStore.getState().setSearchOnTMDB(true)
            route('/search')
          }
        })
        .finally(() => setLoadingResolve(false))
    } else {
      setResolved(null)
    }
  }, [query])

  const loading = loadingSearch || loadingResolve || loadingCurrent

  const handleSearch = (e: SubmitEvent) => {
    e.preventDefault()
    if (!inputValue.trim()) return
    const newParams = new URLSearchParams(window.location.search)
    newParams.set('q', inputValue.trim())
    route(`/admin/validate?${newParams.toString()}`, true)
  }

  const handleMerge = async () => {
    if (!id || !mergeTarget || adding) return
    setAdding(true)
    try {
      await apiFetch(`/titles/${id}/merge`, {
        method: 'POST',
        body: JSON.stringify({ target_id: mergeTarget.id }),
      })
      route(`/title/${mergeTarget.id}`)
    } catch (e) {
      console.error('Merge failed:', e)
      setAdding(false)
    }
  }

  const handleAction = async () => {
    if (adding) return
    setAdding(true)
    try {
      if (id) {
        // Rematch existing title
        const body: RematchPayload = {}
        if (resolved) {
          body.tmdb_id = resolved.tmdb_id ?? undefined
          body.imdb_id = resolved.imdb_id ?? undefined
          body.anilist_id = resolved.anilist_id ?? undefined
          body.type = resolved.type
        }
        
        // If it was just a name search, maybe we have nothing to rematch with IDs
        // but the backend Rematch requires at least one ID.
        // For now, only support rematching if we have resolved metadata.
        if (Object.keys(body).length > 0) {
          await apiFetch(`/titles/${id}/rematch`, {
            method: 'POST',
            body: JSON.stringify(body),
          })
        } else if (!isUrl(query)) {
          // If not an URL and no resolution, maybe we just want to confirm it as is?
          // No, rematch needs IDs. Let's just confirm if no IDs provided.
          await apiFetch(`/titles/${id}`, {
            method: 'PATCH',
            body: JSON.stringify({ match_status: 'confirmed' }),
          })
        }
        
        history.back()
      } else {
        // Add new title
        const body: AddTitlePayload = {
          status: selectedStatus,
          match_status: resolved?.match_status ?? 'unconfirmed',
        }

        if (resolved) {
          body.type = resolved.type
          body.is_anime = resolved.is_anime
          body.year = resolved.release_date ? parseInt(resolved.release_date.slice(0, 4)) : new Date().getFullYear()
          body.names = resolved.names
          body.imdb_id = resolved.imdb_id ?? undefined
          body.tmdb_id = resolved.tmdb_id ?? undefined
          body.tvdb_id = resolved.tvdb_id ?? undefined
          body.anilist_id = resolved.anilist_id ?? undefined
          body.cover_url = resolved.cover_file ? `/covers/${resolved.cover_file}` : null
        } else {
          body.type = 'series'
          body.year = new Date().getFullYear()
          body.names = [{ name: query, language: 'en', is_primary: true }]
        }

        const created = await apiFetch<Title>('/titles', {
          method: 'POST',
          body: JSON.stringify(body),
        })
        route(`/title/${created.id}`)
      }
    } finally {
      setAdding(false)
    }
  }

  const handleRefresh = useCallback(() => { mutateSearch(); mutateCurrent() }, [mutateSearch, mutateCurrent])

  return (
    <PullToRefresh onRefresh={handleRefresh}>
    <div className={s.page}>
      {/* Header */}
      <div className={s.header}>
        <button type="button" onClick={() => history.back()} className={s.backBtn} aria-label={t('common.back')}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke={colors.ink} stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
        <div className={s.headerTitle}>
          {id ? t('validate.fixMatch') : isUrl(query) ? t('validate.addingByUrl') : t('validate.validatingTitle')}
        </div>
      </div>

      {/* Search Bar */}
      <form onSubmit={handleSearch} className={s.searchBar}>
        <input
          type="text"
          value={inputValue}
          onInput={(e) => setInputValue((e.target as HTMLInputElement).value)}
          placeholder={t('validate.searchPlaceholder')}
          className={s.searchInput}
        />
        <button type="submit" className={s.searchBtn}>
          {loadingSearch || loadingResolve ? '...' : t('validate.go')}
        </button>
      </form>

      {loading && !results.length && !resolved && !currentTitle && (
        <div className={s.loading}>
          <div className={s.spinner} />
          {loadingResolve ? t('validate.identifying') : t('validate.matching')}
        </div>
      )}

      {/* Existing title being fixed */}
      {currentTitle && (
        <div className={s.currentSection}>
          <div className={s.sectionLabel}>{t('validate.titleToFix')}</div>
          <div className={s.currentCard}>
            <CoverImage
              coverUrl={currentTitle.cover_url}
              type={currentTitle.type}
              is_anime={currentTitle.is_anime}
              alt={getName(currentTitle)}
              className={s.resultCover}
              iconSize="18px"
            />
            <div className={s.resultInfo}>
              <div className={s.resultNameRow}>
                <span className={s.resultName}>{getName(currentTitle)}</span>
                <StatusBadge status={currentTitle.status} />
              </div>
              <div className={s.resultMeta}>
                {currentTitle.type} · {currentTitle.year}
                {currentTitle.original_title && currentTitle.original_title !== getName(currentTitle) && (
                  <div className={s.originalLabel}>{t('validate.original', { name: currentTitle.original_title })}</div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Existing results (search) */}
      {results.length > 0 && (
        <div className={s.resultsSection}>
          <div className={s.sectionLabel}>
            {t('validate.alreadyInLibrary')}
          </div>
          {results.map((tItem) => (
            <div
              key={tItem.id}
              onClick={() => route(`/title/${tItem.id}`)}
              className={clsx(s.resultCard, tItem.id === Number(id) && s.resultCardCurrent)}
            >
              <CoverImage
                coverUrl={tItem.cover_url}
                type={tItem.type}
                is_anime={tItem.is_anime}
                alt={getName(tItem)}
                className={s.resultCover}
                iconSize="18px"
              />
              <div className={s.resultInfo}>
                <div className={s.resultNameRow}>
                  <span className={s.resultName}>{getName(tItem)}</span>
                  <StatusBadge status={tItem.status} />
                </div>
                <div className={s.resultMeta}>
                  {tItem.type} · {tItem.year}
                </div>
              </div>

              {id && Number(id) !== tItem.id && (
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation()
                    setMergeTarget(tItem)
                  }}
                  className={s.mergeBtn}
                >
                  {t('validate.mergeIntoThis')}
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Manual / External Match Input */}
      {!loading && (
        <div className={s.manualSection}>
        <div className={s.sectionLabel}>
          {id ? t('validate.changeMatch') : t('validate.addNewTitle')}
        </div>

        <div className={s.addCard}>
          <div className={s.addCardTitle}>
            {id ? t('validate.newMatch') : t('validate.addToLibrary')}
          </div>

          {/* Preview of resolved metadata */}
          {resolved && (
            <div className={s.resolvedPreview}>
              <CoverImage
                coverUrl={resolved.cover_file ? `/covers/${resolved.cover_file}` : null}
                type={resolved.type}
                className={s.previewCover}
                iconSize="18px"
              />
              <div className={s.previewInfo}>
                <div className={s.previewName}>{resolved.names.find(n => n.is_primary)?.name || resolved.names[0]?.name}</div>
                <div className={s.previewMeta}>
                  {resolved.type} · {resolved.release_date?.slice(0, 4) || t('validate.unknownYear')}
                </div>
                <div className={s.previewIds}>
                  {!!resolved.imdb_id && <span className={s.idTag}>IMDb</span>}
                  {!!resolved.tmdb_id && <span className={s.idTag}>TMDB</span>}
                  {!!resolved.anilist_id && <span className={s.idTag}>AniList</span>}
                </div>
              </div>
            </div>
          )}

          {!resolved && isUrl(query) && !loading && (
            <div className={s.urlFallback}>
              {t('validate.urlFallback')}
            </div>
          )}

          {/* Status picker (only for new titles) */}
          {!id && (
            <div className={s.statusPicker}>
              {(['watching', 'plan_to_watch', 'completed'] as TitleStatus[]).map((status) => (
                <button
                  key={status}
                  onClick={() => setSelectedStatus(status)}
                  className={clsx(s.statusOption, selectedStatus === status && s.statusOptionSelected)}
                >
                  {status === 'plan_to_watch' ? t('status.planToWatch') : status === 'watching' ? t('status.watching') : t('status.completed')}
                </button>
              ))}
            </div>
          )}

          {/* Block "Add to library" when the URL didn't resolve to a TMDB match —
              creating a title without metadata is dead weight (no edit UI to fix
              it later). The fallback name + redirect handles share flows; this
              guard catches the rare pasted-URL case. */}
          <button
            onClick={handleAction}
            disabled={adding || (!resolved && isUrl(query)) || (!!id && !resolved && !isUrl(query))}
            className={s.addBtn}
          >
            <span className={s.addBtnText}>
              {adding ? (id ? t('validate.updating') : t('validate.adding')) : (id ? t('validate.updateMatch') : t('validate.addToLibrary'))}
            </span>
          </button>
          
          {id && !resolved && !isUrl(query) && (
            <div className={s.hint}>
              {t('validate.hint')}
            </div>
          )}
        </div>
      </div>
      )}

      <ConfirmationDrawer
        open={!!mergeTarget}
        onClose={() => setMergeTarget(null)}
        onConfirm={handleMerge}
        title={t('validate.mergeTitle')}
        description={t('validate.mergeDesc', {
          source: currentTitle ? getName(currentTitle) : '',
          target: mergeTarget ? getName(mergeTarget) : '',
        })}
        confirmText={adding ? t('validate.merging') : t('validate.mergeNow')}
        cancelText={t('common.cancel')}
        isDangerous
      />
    </div>
    </PullToRefresh>
  )
}
