import { useState, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title, TitleStatus, PaginatedResponse, MatchResult } from '../types'
import { useApi } from '../hooks/useApi'
import { getName } from '../utils'
import { StatusBadge } from '../components/StatusBadge'
import { apiFetch } from '../api'
import { CoverPlaceholder, coverBackground } from '../components/CoverPlaceholder'
import clsx from 'clsx'
import s from './Validate.module.css'

function isUrl(str: string): boolean {
  return /^(https?:\/\/)?([\w.-]+)+(\/[\w.-]*)*\/?/i.test(str)
}

export function Validate({ path }: { path?: string }) {
  const params = new URLSearchParams(window.location.search)
  const query = params.get('q') ?? ''
  const searchPath = query ? `/titles?search=${encodeURIComponent(query)}` : null
  const { data: resultsData, loading: loadingSearch } = useApi<PaginatedResponse>(searchPath)
  const results = resultsData?.titles ?? []
  
  const [adding, setAdding] = useState(false)
  const [selectedStatus, setSelectedStatus] = useState<TitleStatus>('plan_to_watch')
  const [resolved, setResolved] = useState<MatchResult | null>(null)
  const [loadingResolve, setLoadingResolve] = useState(false)

  useEffect(() => {
    if (isUrl(query)) {
      setLoadingResolve(true)
      apiFetch<MatchResult>(`/titles/resolve?q=${encodeURIComponent(query)}`)
        .then(setResolved)
        .catch(() => {})
        .finally(() => setLoadingResolve(false))
    }
  }, [query])

  const loading = loadingSearch || loadingResolve

  const handleAdd = async () => {
    if (adding) return
    setAdding(true)
    try {
      const body: any = {
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
    } finally {
      setAdding(false)
    }
  }

  return (
    <div className={s.page}>
      {/* Header */}
      <div className={s.header}>
        <div onClick={() => history.back()} className={s.backBtn}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <line x1="19" y1="12" x2="5" y2="12" /><polyline points="12 19 5 12 12 5" />
          </svg>
        </div>
        <div className={s.headerTitle}>
          {isUrl(query) ? 'Adding by URL' : `Validating: ${query}`}
        </div>
      </div>

      {loading && (
        <div className={s.loading}>
          <div className={s.spinner} />
          {loadingResolve ? 'Identifying...' : 'Matching...'}
        </div>
      )}

      {/* Existing results */}
      {results.length > 0 && (
        <div className={s.resultsSection}>
          <div className={s.sectionLabel}>
            Already in library
          </div>
          {results.map((t) => (
            <div
              key={t.id}
              onClick={() => route(`/title/${t.id}`)}
              className={s.resultCard}
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
            </div>
          ))}
        </div>
      )}

      {/* Add new */}
      {!loading && (
        <div className={s.addCard}>
          <div className={s.addCardTitle}>
            Add as new title
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

          {/* Status picker */}
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

          <button
            onClick={handleAdd}
            disabled={adding}
            className={s.addBtn}
          >
            <span className={s.addBtnText}>
              {adding ? 'Adding...' : 'Add to library'}
            </span>
          </button>
        </div>
      )}
    </div>
  )
}
