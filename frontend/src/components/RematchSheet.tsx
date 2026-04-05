import { useState } from 'preact/hooks'
import type { Title } from '../types'
import { apiFetch } from '../api'
import { getName } from '../utils'
import { BottomSheet } from './BottomSheet'
import s from './RematchSheet.module.css'

interface RematchSheetProps {
  open: boolean
  onClose: () => void
  title: Title
  onDone: () => void
}

interface TMDBResult {
  id: number
  title: string
  year: number
  poster_url: string | null
}

export function RematchSheet({ open, onClose, title, onDone }: RematchSheetProps) {
  const [query, setQuery] = useState(getName(title))
  const [mediaType, setMediaType] = useState<'movie' | 'tv'>(title.type === 'movie' ? 'movie' : 'tv')
  const [results, setResults] = useState<TMDBResult[]>([])
  const [hasSearched, setHasSearched] = useState(false)
  const [searching, setSearching] = useState(false)
  const [saving, setSaving] = useState(false)
  const [showManual, setShowManual] = useState(false)
  const [manualTmdb, setManualTmdb] = useState('')
  const [manualImdb, setManualImdb] = useState('')
  const [manualAnilist, setManualAnilist] = useState('')
  const doSearch = async (q: string, type: 'movie' | 'tv') => {
    if (!q.trim()) {
      setResults([])
      return
    }
    setSearching(true)
    setHasSearched(true)
    try {
      const data = await apiFetch<TMDBResult[]>(`/tmdb/search?query=${encodeURIComponent(q)}&type=${type}`)
      setResults(data)
    } catch {
      setResults([])
    } finally {
      setSearching(false)
    }
  }

  const handleSubmit = () => doSearch(query, mediaType)

  const handleKeyDown = (e: KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      handleSubmit()
    }
  }

  const handleTypeChange = (type: 'movie' | 'tv') => {
    setMediaType(type)
    doSearch(query, type)
  }

  const handleSelect = async (result: TMDBResult) => {
    setSaving(true)
    try {
      await apiFetch(`/titles/${title.id}/rematch`, {
        method: 'POST',
        body: JSON.stringify({ tmdb_id: result.id }),
      })
      onDone()
      onClose()
    } finally {
      setSaving(false)
    }
  }

  const handleManualSave = async () => {
    const body: Record<string, unknown> = {}
    if (manualTmdb) body.tmdb_id = parseInt(manualTmdb, 10)
    if (manualImdb) body.imdb_id = manualImdb
    if (manualAnilist) body.anilist_id = parseInt(manualAnilist, 10)
    if (Object.keys(body).length === 0) return

    setSaving(true)
    try {
      await apiFetch(`/titles/${title.id}/rematch`, {
        method: 'POST',
        body: JSON.stringify(body),
      })
      onDone()
      onClose()
    } finally {
      setSaving(false)
    }
  }

  return (
    <BottomSheet open={open} onClose={onClose}>
      <div className={s.content}>
        {/* Search bar */}
        <div className={s.searchRow}>
          <input
            type="text"
            value={query}
            onInput={(e) => setQuery((e.target as HTMLInputElement).value)}
            onKeyDown={handleKeyDown}
            placeholder="Search TMDB..."
            className={s.searchInput}
            autoFocus
          />
          <button onClick={handleSubmit} disabled={searching} className={s.searchBtn} aria-label="Search">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="11" cy="11" r="8" />
              <line x1="21" y1="21" x2="16.65" y2="16.65" />
            </svg>
          </button>
        </div>

        {/* Type toggle */}
        <div className={s.typeToggle}>
          {(['movie', 'tv'] as const).map((t) => (
            <button
              key={t}
              onClick={() => handleTypeChange(t)}
              className={`${s.typeBtn} ${mediaType === t ? s.active : ''}`}
            >
              {t === 'movie' ? 'Movie' : 'TV'}
            </button>
          ))}
        </div>

        {/* Results grid */}
        <div className={s.results}>
          {searching && <div className={s.status}>Searching...</div>}
          {!searching && results.length === 0 && hasSearched && (
            <div className={s.status}>No results</div>
          )}
          {results.map((r) => (
            <button
              key={r.id}
              onClick={() => handleSelect(r)}
              disabled={saving}
              className={s.resultCard}
            >
              <div className={s.poster}>
                {r.poster_url ? (
                  <img src={r.poster_url} alt={r.title} loading="lazy" />
                ) : (
                  <div className={s.noPoster}>?</div>
                )}
              </div>
              <div className={s.resultInfo}>
                <div className={s.resultTitle}>{r.title}</div>
                {r.year > 0 && <div className={s.resultYear}>{r.year}</div>}
              </div>
            </button>
          ))}
        </div>

        {/* Manual IDs section */}
        <button onClick={() => setShowManual(!showManual)} className={s.manualToggle}>
          {showManual ? '▾' : '▸'} Manual IDs
        </button>

        {showManual && (
          <div className={s.manualSection}>
            <label className={s.fieldLabel}>
              TMDB ID
              <input type="text" value={manualTmdb} onInput={(e) => setManualTmdb((e.target as HTMLInputElement).value)} className={s.fieldInput} placeholder="e.g. 550" />
            </label>
            <label className={s.fieldLabel}>
              IMDB ID
              <input type="text" value={manualImdb} onInput={(e) => setManualImdb((e.target as HTMLInputElement).value)} className={s.fieldInput} placeholder="e.g. tt0137523" />
            </label>
            <label className={s.fieldLabel}>
              AniList ID
              <input type="text" value={manualAnilist} onInput={(e) => setManualAnilist((e.target as HTMLInputElement).value)} className={s.fieldInput} placeholder="e.g. 21" />
            </label>
            <button onClick={handleManualSave} disabled={saving} className={s.saveButton}>
              <span className={s.saveButtonLabel}>{saving ? 'Saving...' : 'Save & re-enrich'}</span>
            </button>
          </div>
        )}
      </div>
    </BottomSheet>
  )
}
