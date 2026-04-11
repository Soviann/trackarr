import { useState, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title, TitleStatus, PaginatedResponse, MatchResult } from '../types'
import { useApi } from '../hooks/useApi'
import { getName } from '../utils'
import { StatusBadge } from '../components/StatusBadge'
import { apiFetch } from '../api'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import { ConfirmationDrawer } from '../components/ConfirmationDrawer'
import clsx from 'clsx'
import s from './Validate.module.css'

function isUrl(str: string): boolean {
  return /^(https?:\/\/)?([\w.-]+)+\.([a-z]{2,10})(\/[\w.-]*)*\/?$/i.test(str)
}

interface RematchPayload {
  tmdb_id?: number
  imdb_id?: string
  anilist_id?: number
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
  const params = new URLSearchParams(window.location.search)
  const query = params.get('q') ?? ''
  const id = params.get('id')
  
  const searchPath = query && !isUrl(query) ? `/titles?search=${encodeURIComponent(query)}` : null
  const { data: resultsData, loading: loadingSearch } = useApi<PaginatedResponse>(searchPath)
  const results = resultsData?.titles ?? []

  const { data: currentTitle, loading: loadingCurrent } = useApi<Title>(id ? `/titles/${id}` : null)
  
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
          body.tmdb_id = resolved.tmdb_id
          body.imdb_id = resolved.imdb_id
          body.anilist_id = resolved.anilist_id
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
          body.imdb_id = resolved.imdb_id
          body.tmdb_id = resolved.tmdb_id
          body.tvdb_id = resolved.tvdb_id
          body.anilist_id = resolved.anilist_id
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

  return (
    <div className={s.page}>
      {/* Header */}
      <div className={s.header}>
        <button type="button" onClick={() => history.back()} className={s.backBtn} aria-label="Back">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
        </button>
        <div className={s.headerTitle}>
          {id ? 'Fix match' : isUrl(query) ? 'Adding by URL' : 'Validating title'}
        </div>
      </div>

      {/* Search Bar */}
      <form onSubmit={handleSearch} className={s.searchBar}>
        <input
          type="text"
          value={inputValue}
          onInput={(e) => setInputValue((e.target as HTMLInputElement).value)}
          placeholder="Search name or paste TMDB/IMDb/AniList URL..."
          className={s.searchInput}
        />
        <button type="submit" className={s.searchBtn}>
          {loadingSearch || loadingResolve ? '...' : 'Go'}
        </button>
      </form>

      {loading && !results.length && !resolved && !currentTitle && (
        <div className={s.loading}>
          <div className={s.spinner} />
          {loadingResolve ? 'Identifying...' : 'Matching...'}
        </div>
      )}

      {/* Existing title being fixed */}
      {currentTitle && (
        <div className={s.currentSection}>
          <div className={s.sectionLabel}>Title to fix</div>
          <div className={s.currentCard}>
            <div
              className={s.resultCover}
              style={{ background: coverBackground(currentTitle.cover_url, currentTitle.type) }}
            >
              {!currentTitle.cover_url && <CoverPlaceholder type={currentTitle.type} iconSize="18px" />}
            </div>
            <div className={s.resultInfo}>
              <div className={s.resultNameRow}>
                <span className={s.resultName}>{getName(currentTitle)}</span>
                <StatusBadge status={currentTitle.status} />
              </div>
              <div className={s.resultMeta}>
                {currentTitle.type} · {currentTitle.year}
                {currentTitle.original_title && currentTitle.original_title !== getName(currentTitle) && (
                  <div className={s.originalLabel}>Plex: {currentTitle.original_title}</div>
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
            Already in library
          </div>
          {results.map((t) => (
            <div
              key={t.id}
              onClick={() => route(`/title/${t.id}`)}
              className={clsx(s.resultCard, t.id === Number(id) && s.resultCardCurrent)}
            >
              <div
                className={s.resultCover}
                style={{ background: coverBackground(t.cover_url, t.type) }}
              >
                {!t.cover_url && <CoverPlaceholder type={t.type} iconSize="18px" />}
              </div>
              <div className={s.resultInfo}>
                <div className={s.resultNameRow}>
                  <span className={s.resultName}>{getName(t)}</span>
                  <StatusBadge status={t.status} />
                </div>
                <div className={s.resultMeta}>
                  {t.type} · {t.year}
                </div>
              </div>

              {id && t.id !== Number(id) && (
                <button
                  className={s.mergeBtn}
                  onClick={(e) => {
                    e.stopPropagation()
                    setMergeTarget(t)
                  }}
                >
                  Merge
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Action card */}
      {!loading && (
        <div className={s.addCard}>
          <div className={s.addCardTitle}>
            {id ? 'New Match' : 'Add to library'}
          </div>

          {/* Preview of resolved metadata */}
          {resolved && (
            <div className={s.resolvedPreview}>
              <div 
                className={s.previewCover}
                style={{ background: coverBackground(resolved.cover_file ? `/covers/${resolved.cover_file}` : null, resolved.type) }}
              >
                {!resolved.cover_file && <CoverPlaceholder type={resolved.type} iconSize="18px" />}
              </div>
              <div className={s.previewInfo}>
                <div className={s.previewName}>{resolved.names.find(n => n.is_primary)?.name || resolved.names[0]?.name}</div>
                <div className={s.previewMeta}>
                  {resolved.type} · {resolved.release_date?.slice(0, 4) || 'Unknown year'}
                </div>
                <div className={s.previewIds}>
                  {resolved.imdb_id && <span className={s.idTag}>IMDb</span>}
                  {resolved.tmdb_id && <span className={s.idTag}>TMDB</span>}
                  {resolved.anilist_id && <span className={s.idTag}>AniList</span>}
                </div>
              </div>
            </div>
          )}

          {!resolved && isUrl(query) && !loading && (
            <div className={s.urlFallback}>
              Could not identify title from URL. It will be added with the URL as name.
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
                  {status === 'plan_to_watch' ? 'Plan to watch' : status.charAt(0).toUpperCase() + status.slice(1)}
                </button>
              ))}
            </div>
          )}

          <button
            onClick={handleAction}
            disabled={adding || (!!id && !resolved && !isUrl(query))}
            className={s.addBtn}
          >
            <span className={s.addBtnText}>
              {adding ? (id ? 'Updating...' : 'Adding...') : (id ? 'Update match' : 'Add to library')}
            </span>
          </button>
          
          {id && !resolved && !isUrl(query) && (
            <div className={s.hint}>
              Search above or paste an URL (TMDB/AniList) to fix the match for this title.
            </div>
          )}
        </div>
      )}

      <ConfirmationDrawer
        open={!!mergeTarget}
        onClose={() => setMergeTarget(null)}
        onConfirm={handleMerge}
        title="Merge titles?"
        description={`This will merge "${currentTitle ? getName(currentTitle) : 'this title'}" into "${mergeTarget ? getName(mergeTarget) : ''}". Seasons, watch events and names will be moved. This action cannot be undone.`}
        confirmText={adding ? 'Merging...' : 'Merge now'}
        isDangerous
      />
    </div>
  )
}
