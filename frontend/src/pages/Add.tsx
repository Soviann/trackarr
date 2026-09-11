import { useState, useRef, useEffect } from 'preact/hooks'
import { route } from 'preact-router'
import type { Title, TitleStatus } from '../types'
import { apiFetch } from '../api'
import { getName } from '../utils'
import { detectUrlType } from '../utils/url'
import { routeTo } from '../routes'
import { useTranslation } from '../i18n'
import { useUndo } from '../context/UndoContext'
import { useSearchStore } from '../store'
import { CoverImage } from '../components/CoverImage'
import s from './Add.module.css'

interface SearchItem {
  id: string
  title: string
  year: number
  type: 'movie' | 'series'
  isAnime: boolean
  posterUrl: string | null
  overview: string
  source: 'TMDB' | 'AniList'
  tmdbId?: number
  anilistId?: number
  localTitleId?: number
  adding?: boolean
}

// extractSharedUrl scans share-target params for the first http(s) URL.
function extractSharedUrl(params: URLSearchParams): string | null {
  for (const key of ['url', 'text', 'title']) {
    const v = params.get(key)
    if (!v) continue
    const m = v.match(/https?:\/\/\S+/i)
    if (m) return m[0]
  }
  return null
}

// extractSharedName recovers a human title for TMDB search when the URL
// resolution fails. IMDb on Android sends EXTRA_SUBJECT (`title`) as the bare
// movie name; AniList sends similar. Strips trailing " - IMDb" / "- AniList".
function extractSharedName(params: URLSearchParams): string | null {
  const fromTitle = params.get('title')?.trim()
  if (fromTitle && !/^https?:\/\//i.test(fromTitle)) {
    return fromTitle.replace(/\s*[-–|·]\s*(IMDb|AniList)\s*$/i, '').trim() || null
  }
  const fromText = params.get('text')?.trim()
  if (fromText) {
    const stripped = fromText.replace(/https?:\/\/\S+/gi, '').trim()
    if (stripped) return stripped.replace(/\s*[-–|·]\s*(IMDb|AniList)\s*$/i, '').trim() || null
  }
  return null
}

export function Add({ path: _ }: { path?: string }) {
  const { t } = useTranslation()
  const { showUndo } = useUndo()
  const [query, setQuery] = useState('')
  const [results, setResults] = useState<SearchItem[]>([])
  const [loading, setLoading] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    inputRef.current?.focus()

    const params = new URLSearchParams(typeof window !== 'undefined' ? window.location.search : '')
    const sharedUrl = extractSharedUrl(params)
    const sharedName = extractSharedName(params)

    if (sharedUrl) {
      setQuery(sharedUrl)
      const target = sharedName
        ? `/admin/validate?q=${encodeURIComponent(sharedUrl)}&name=${encodeURIComponent(sharedName)}`
        : `/admin/validate?q=${encodeURIComponent(sharedUrl)}`
      route(target)
      return
    }
    if (sharedName) {
      setQuery(sharedName)
      useSearchStore.getState().setQuery(sharedName)
      useSearchStore.getState().setSearchOnTMDB(true)
    }
  }, [])

  const urlType = query.trim() ? detectUrlType(query.trim()) : null

  // Debounced live search across TMDB, AniList, and local library
  useEffect(() => {
    const trimmed = query.trim()
    if (!trimmed || urlType) {
      setResults([])
      setLoading(false)
      return
    }

    let cancelled = false
    setLoading(true)

    const timer = setTimeout(async () => {
      try {
        const [tmdbRes, anilistRes, localRes] = await Promise.allSettled([
          apiFetch<Array<{ id: number; title: string; year: number; poster_url: string | null; type?: string; overview?: string }>>(
            `/tmdb/search?query=${encodeURIComponent(trimmed)}&type=all`
          ),
          apiFetch<Array<{ id: number; title: string; year?: number | null; format?: string; poster_url?: string | null; english_title?: string; romaji_title?: string }>>(
            `/anilist/search?query=${encodeURIComponent(trimmed)}`
          ),
          apiFetch<{ titles: Title[] }>(`/titles?q=${encodeURIComponent(trimmed)}`),
        ])

        if (cancelled) return

        const localTitles: Title[] = localRes.status === 'fulfilled' && localRes.value && localRes.value.titles ? localRes.value.titles : []

        const items: SearchItem[] = []

        // Process TMDB results
        if (tmdbRes.status === 'fulfilled' && Array.isArray(tmdbRes.value)) {
          for (const tmdb of tmdbRes.value) {
            const isSeries = tmdb.type === 'tv'
            const item: SearchItem = {
              id: `tmdb-${tmdb.id}`,
              title: tmdb.title,
              year: tmdb.year,
              type: isSeries ? 'series' : 'movie',
              isAnime: false,
              posterUrl: tmdb.poster_url,
              overview: tmdb.overview ?? '',
              source: 'TMDB',
              tmdbId: tmdb.id,
            }

            const localHit = localTitles.find((t) => {
              if (t.tmdb_id && t.tmdb_id === tmdb.id) return true
              const localName = getName(t).toLowerCase()
              const searchName = tmdb.title.toLowerCase()
              return localName === searchName && (!tmdb.year || !t.year || t.year === tmdb.year)
            })
            if (localHit) item.localTitleId = localHit.id

            items.push(item)
          }
        }

        // Process AniList results
        if (anilistRes.status === 'fulfilled' && Array.isArray(anilistRes.value)) {
          for (const al of anilistRes.value) {
            const itemTitle = al.title || al.english_title || al.romaji_title || ''
            if (!itemTitle) continue

            // Deduplicate if already present from TMDB by title and year
            const duplicate = items.some(
              (it) => it.title.toLowerCase() === itemTitle.toLowerCase() && it.year === (al.year ?? 0)
            )
            if (duplicate) continue

            const isMovie = al.format === 'MOVIE'
            const item: SearchItem = {
              id: `anilist-${al.id}`,
              title: itemTitle,
              year: al.year ?? 0,
              type: isMovie ? 'movie' : 'series',
              isAnime: true,
              posterUrl: al.poster_url ?? null,
              overview: '',
              source: 'AniList',
              anilistId: al.id,
            }

            const localHit = localTitles.find((t) => {
              if (t.anilist_id && t.anilist_id === al.id) return true
              const localName = getName(t).toLowerCase()
              const searchName = itemTitle.toLowerCase()
              return localName === searchName && (!al.year || !t.year || t.year === al.year)
            })
            if (localHit) item.localTitleId = localHit.id

            items.push(item)
          }
        }

        setResults(items)
      } catch {
        if (!cancelled) setResults([])
      } finally {
        if (!cancelled) setLoading(false)
      }
    }, 300)

    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [query, urlType])

  const handleSubmit = () => {
    if (!query.trim()) return
    route(`/admin/validate?q=${encodeURIComponent(query.trim())}`)
  }

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter') handleSubmit()
  }

  const handleAdd = async (item: SearchItem, status: TitleStatus) => {
    if (item.localTitleId || item.adding) return
    setResults((prev) => prev.map((r) => (r.id === item.id ? { ...r, adding: true } : r)))
    try {
      const created = await apiFetch<Title>('/titles', {
        method: 'POST',
        body: JSON.stringify({
          type: item.type,
          is_anime: item.isAnime,
          year: item.year,
          status,
          match_status: 'confirmed',
          names: [{ name: item.title, language: 'en', is_primary: true }],
          cover_url: item.posterUrl,
          tmdb_id: item.tmdbId,
          anilist_id: item.anilistId,
        }),
      })

      setResults((prev) =>
        prev.map((r) => (r.id === item.id ? { ...r, localTitleId: created.id, adding: false } : r))
      )

      showUndo({
        message: t('undo.titleAdded', { title: item.title }),
        onUndo: async () => {
          setResults((prev) =>
            prev.map((r) => (r.id === item.id ? { ...r, localTitleId: undefined } : r))
          )
          await apiFetch(`/titles/${created.id}`, { method: 'DELETE' })
        },
      })
    } catch {
      setResults((prev) => prev.map((r) => (r.id === item.id ? { ...r, adding: false } : r)))
    }
  }

  return (
    <div className={s.page}>
      {/* Top Search Bar */}
      <div className={s.searchBar}>
        <div className={s.inputWrapper}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="var(--ink-dim)" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="11" cy="11" r="8" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <input
            ref={inputRef}
            type="text"
            name="add-title"
            id="add-title"
            autocomplete="off"
            value={query}
            onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
            onKeyDown={handleKeyDown}
            placeholder={t('add.placeholder')}
            className={s.input}
          />
          {query && (
            <button
              type="button"
              onClick={() => { setQuery(''); setResults([]) }}
              className={s.clearBtn}
              aria-label="Clear"
            >
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          )}
          {urlType && (
            <button onClick={handleSubmit} className={s.goBtn}>
              {t('add.go')}
            </button>
          )}
        </div>
      </div>

      {/* URL Detection Banner */}
      {urlType && (
        <div className={s.center}>
          <div className={s.centerContent}>
            <div className={s.urlBadge}>
              {t('add.urlDetected', { type: urlType.toUpperCase() })}
            </div>
            <p className={s.emptyText}>
              {t('add.pressEnter', { query: query.trim() })}
            </p>
          </div>
        </div>
      )}

      {/* Empty Initial State */}
      {!query.trim() && (
        <div className={s.center}>
          <div className={s.centerContent}>
            <div className={s.emptyIcon}>
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--ink-mute)" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="12" cy="12" r="10" />
                <line x1="12" y1="8" x2="12" y2="16" />
                <line x1="8" y1="12" x2="16" y2="12" />
              </svg>
            </div>
            <div className={s.emptyText}>{t('add.emptyTitle')}</div>
            <div className={s.emptyHint}>{t('add.emptyHint')}</div>
          </div>
        </div>
      )}

      {/* Loading Indicator */}
      {loading && query.trim() && !urlType && (
        <div className={s.loadingNotice}>
          {t('add.searching')}
        </div>
      )}

      {/* Search Results List */}
      {!urlType && results.length > 0 && (
        <div className={s.resultsList}>
          {results.map((r) => (
            <div key={r.id} className={s.resultCard}>
              <div className={s.posterWrap}>
                <CoverImage
                  coverUrl={r.posterUrl}
                  type={r.type}
                  is_anime={r.isAnime}
                  className={s.poster}
                  alt={r.title}
                />
              </div>

              <div className={s.resultInfo}>
                <div className={s.titleRow}>
                  <span className={s.titleName} title={r.title}>{r.title}</span>
                  <span className={s.sourceBadge}>{r.source}</span>
                </div>

                <div className={s.subRow}>
                  {r.type === 'movie' ? t('add.movie') : r.isAnime ? t('add.anime') : t('add.series')} {r.year > 0 ? `• ${r.year}` : ''}
                </div>

                {r.overview && (
                  <p className={s.overview}>{r.overview}</p>
                )}

                <div className={s.actionsRow}>
                  {r.localTitleId ? (
                    <a
                      href={routeTo.title(r.localTitleId)}
                      className={s.inLibraryLink}
                      onClick={(e) => {
                        e.preventDefault()
                        route(routeTo.title(r.localTitleId!))
                      }}
                    >
                      {t('add.inLibrary')} ↗
                    </a>
                  ) : (
                    <>
                      <button
                        type="button"
                        disabled={r.adding}
                        onClick={() => handleAdd(r, 'plan_to_watch')}
                        className={s.btnAction}
                      >
                        {r.adding ? '...' : t('add.planToWatch')}
                      </button>
                      <button
                        type="button"
                        disabled={r.adding}
                        onClick={() => handleAdd(r, 'watching')}
                        className={`${s.btnAction} ${s.btnActionPrimary}`}
                      >
                        {r.adding ? '...' : t('add.watching')}
                      </button>
                    </>
                  )}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* No results message */}
      {!loading && !urlType && query.trim() && results.length === 0 && (
        <div className={s.center}>
          <div className={s.centerContent}>
            <div className={s.emptyText}>
              {t('add.noResults', { query: query.trim() })}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

